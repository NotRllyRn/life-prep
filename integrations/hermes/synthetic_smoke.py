"""Operator-only synthetic E2E fixture. Reads no mailbox; no secrets in output.
Run with private --root, --secret-env-file and --discord-env-file.
Creates only synthetic prep and notices/replies/threads in --channel.
"""
import argparse
from contextlib import closing
import hashlib
import hmac
import json
import os
from pathlib import Path
import sqlite3
import time
import urllib.request
import uuid


def env_file(path):
    from dotenv import dotenv_values
    return dotenv_values(path)


def post(url, payload, headers):
    request = urllib.request.Request(url, data=json.dumps(payload).encode(), headers={'Content-Type':'application/json', **headers}, method='POST')
    with urllib.request.urlopen(request, timeout=45) as response:
        return response.status, json.load(response)


def webhook(secret, event, source, extra=None):
    payload = {'event_type':'life-prep.context', 'event_id':event, 'context':{'source_ref':source}, **(extra or {})}
    body = json.dumps(payload).encode()
    timestamp = str(int(time.time()))
    signature = hmac.new(secret.encode(),timestamp.encode()+b'.'+body,hashlib.sha256).hexdigest()
    request = urllib.request.Request('http://127.0.0.1:8644/webhooks/life-prep',data=body,headers={'Content-Type':'application/json','X-Webhook-Timestamp':timestamp,'X-Webhook-Signature-V2':signature,'X-Request-ID':event if not extra else 'probe-'+uuid.uuid4().hex},method='POST')
    with urllib.request.urlopen(request,timeout=30) as response:
        return response.status,json.load(response)


def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--root',required=True)
    ap.add_argument('--channel',required=True)
    ap.add_argument('--secret-env-file',required=True)
    ap.add_argument('--discord-env-file',required=True)
    ap.add_argument('--phase',choices=['prepare','resolve'],required=True)
    ap.add_argument('--deterministic',action='store_true',help='Use opt-in native synthetic fixture instead of an LLM preparation run')
    a=ap.parse_args()
    root=Path(a.root).resolve()
    secret=env_file(a.secret_env_file)['LIFE_PREP_WEBHOOK_SECRET']
    manifest=root/'context-smoke.json'
    if a.phase=='prepare':
        cases=[]
        for label in ['A','B']:
            source_id='context-synthetic-'+label+'-'+uuid.uuid4().hex
            event=hashlib.sha256(('activation-synthetic\0'+source_id).encode()).hexdigest()
            body={'ID':source_id,'Subject':'Synthetic exact-item context '+label,'Body':'Benign synthetic readiness test only. Prepare an event-specific local checklist and plan for validating reply/thread context. Do not read actual mail.'}
            source=root/'artifacts'/(event+'.json')
            source.write_text(json.dumps({'EventID':event,'Account':'activation-synthetic','Message':body}));source.chmod(0o600)
            with closing(sqlite3.connect(root/'state.db',timeout=5)) as db, db:
                db.execute("INSERT INTO events(id,account,state,body,artifact) VALUES(?,?,'dispatching',?,?)",(event,'activation-synthetic',json.dumps(body),str(source)))
            status,result=webhook(secret,event,str(source),{'life_prep_synthetic_prepare':True} if a.deterministic else None)
            with closing(sqlite3.connect(root/'state.db',timeout=5)) as db, db:
                db.execute("UPDATE events SET state=?,error=? WHERE id=?",('accepted' if status==202 else 'uncertain','Synthetic smoke dispatch HTTP '+str(status),event))
            cases.append(dict(label=label,event_id=event,source_ref=str(source),http=status))
            print(label,event,status,flush=True)
        manifest.write_text(json.dumps(cases,indent=2));manifest.chmod(0o600)
        return
    cases=json.loads(manifest.read_text())
    # Wait for genuine agent-generated preparation and verified notice mapping.
    deadline=time.monotonic()+600
    while True:
        with closing(sqlite3.connect(root/'notice-context.db')) as db:
            for case in cases:
                row=db.execute('SELECT message,context FROM notices WHERE channel=? AND event=? ORDER BY rowid LIMIT 1',(a.channel,case['event_id'])).fetchone()
                if row: case.update(notice_id=row[0],context=json.loads(row[1]))
        if all('notice_id' in c for c in cases):break
        if time.monotonic()>deadline:raise RuntimeError('notice mapping missing; do not blindly resend')
        time.sleep(5)
    token=env_file(a.discord_env_file).get('DISCORD_BOT_TOKEN') or os.environ.get('DISCORD_BOT_TOKEN')
    if not token:raise RuntimeError('existing Discord bot token unavailable')
    headers={'Authorization':'Bot '+token,'User-Agent':'life-prep-context-smoke (operator synthetic verification)'}
    base='https://discord.com/api/v10'
    probes=[]
    for case in cases:
        with urllib.request.urlopen(urllib.request.Request(base+'/channels/'+a.channel+'/messages/'+case['notice_id'],headers=headers),timeout=30) as response:notice=json.load(response)
        assert notice['id']==case['notice_id'] and notice['channel_id']==a.channel
        _,reply=post(base+'/channels/'+a.channel+'/messages',{'content':'Synthetic context verification '+case['label']+': expand / brief (bot test, not a human request).','allowed_mentions':{'parse':[]},'message_reference':{'message_id':case['notice_id'],'channel_id':a.channel,'fail_if_not_exists':True}},headers)
        _,thread=post(base+'/channels/'+a.channel+'/messages/'+case['notice_id']+'/threads',{'name':'Synthetic life-prep context '+case['label'],'auto_archive_duration':60},headers)
        _,message=post(base+'/channels/'+thread['id']+'/messages',{'content':'Synthetic context verification: brief / expand (bot test).','allowed_mentions':{'parse':[]}},headers)
        probes += [dict(channel_id=a.channel,message_id=reply['id']),dict(channel_id=thread['id'],message_id=message['id'])]
        case.update(reply_id=reply['id'],thread_id=thread['id'],thread_message_id=message['id'])
    probes.append(dict(channel_id=a.channel,message_id=cases[0]['reply_id'],unknown=True))
    status,result=webhook(secret,cases[0]['event_id'],cases[0]['source_ref'],{'life_prep_probe':probes})
    assert status==202
    evidence=root/('context-probe-'+cases[0]['event_id']+'.json')
    deadline=time.monotonic()+60
    while not evidence.exists():
        if time.monotonic()>deadline:raise RuntimeError('native gateway probe did not produce evidence')
        time.sleep(1)
    results=json.loads(evidence.read_text())
    assert len(results)==5
    for index,row in enumerate(results[:4]):
        expected=cases[index//2]['event_id']; other=cases[1-index//2]['event_id']
        assert expected in row['resolved_text'] and other not in row['resolved_text'],row
    assert 'Do not use latest' in results[4]['resolved_text']
    assert not any(c['event_id'] in results[4]['resolved_text'] for c in cases)
    manifest.write_text(json.dumps(cases,indent=2))
    print(json.dumps({'native_reply_cases':2,'native_thread_cases':2,'unknown_failed_closed':True,'cases':[{k:v for k,v in c.items() if k!='context'} for c in cases],'evidence':str(evidence)},indent=2))

if __name__=='__main__':main()
