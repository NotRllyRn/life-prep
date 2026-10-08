"""Offline wrapper -> actual Go receipt -> SQLite readback; no network/mail."""
import asyncio
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tempfile
from types import SimpleNamespace as NS
import unittest
from unittest.mock import patch
from test_context import load
from gateway.platforms.base import SendResult

class CompletionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        binary = os.environ['LIFE_PREP_TEST_BINARY']
        shutil.copy2(binary, self.root / 'life-prep')
        subprocess.run([binary, 'demo', str(self.root)], check=True, capture_output=True)
        self.config = self.root / 'config.json'
        self.config.write_text(json.dumps({'Database':str(self.root/'state.db'), 'Artifacts':str(self.root/'artifacts')}))
        with sqlite3.connect(self.root/'state.db') as db:
            self.id, self.source = db.execute('select id,artifact from events').fetchone()
            db.execute("update events set state='accepted',account='primary'")
        self.folder = self.root/'prepared'/self.id
        self.folder.mkdir(parents=True)
        self.plan = self.folder/'plan.md'
        self.plan.write_text('Fixture obligation: submit the local draft before the stated deadline; external submission requires approval.')
        self.outcome = dict(event_id=self.id,source_ref=self.source,status='prepared',summary='Deadline and draft prepared from this source.')
        self.save_outcome()
        self.chat = 'webhook:life-prep:'+self.id
        self.content = self.outcome['summary']+' Event '+self.id+' '+str(self.plan)
        self.calls = []
        self.readback = None
        async def deliver(chat, content, **kw):
            self.calls.append((chat,content))
            return SendResult(success=True,message_id='111')
        async def fetch(mid):
            if isinstance(self.readback, Exception): raise self.readback
            return NS(id=mid,channel=NS(id=123),author=NS(id=99),content=self.content if self.readback is None else self.readback)
        self.webhook=NS(send=deliver,_delivery_info={self.chat:{'deliver':'discord','deliver_extra':{'chat_id':'123'}}})
        self.gateway=NS(adapters={'webhook':self.webhook,'discord':NS(_client=NS(user=NS(id=99),get_channel=lambda _:NS(fetch_message=fetch)))})
        self.bridge=load().Bridge(self.root,'123')
        self.bridge.install(self.gateway)
        self.mirror=patch('gateway.mirror.mirror_to_session',return_value=False)
        self.mirror.start(); self.addCleanup(self.mirror.stop)

    def save_outcome(self):
        (self.folder/'outcome.json').write_text(json.dumps(self.outcome))

    def state(self):
        with sqlite3.connect(self.root/'state.db') as db:
            return db.execute('select state,result from events where id=?',(self.id,)).fetchone()

    def send(self,content=None):
        return asyncio.run(self.webhook.send(self.chat,self.content if content is None else content))

    def test_verified_receipt_and_duplicate_dispatch_and_delivery(self):
        self.assertTrue(self.send().success)
        state,result=self.state()
        self.assertEqual(state,'completed')
        evidence=json.loads(Path(result).read_text())
        self.assertEqual(evidence['notices'][0]['notice_text'],self.content)
        self.assertEqual(evidence['notices'][0]['notice_message_id'],'111')
        self.send()
        self.assertEqual(len(self.calls),1)
        event=NS(source=NS(platform=NS(value='webhook'),chat_id=self.chat),raw_message={'event_id':self.id})
        self.assertEqual(self.bridge.dispatch(event,self.gateway)['action'],'skip')
        restarted=load().Bridge(self.root,'123')
        self.assertTrue(restarted.completed(self.id))

    def test_failed_readback_text_or_transport_never_completes(self):
        for readback in ['different bot message',RuntimeError('offline readback failed')]:
            with self.subTest(readback=str(readback)), self.assertLogs(level='ERROR'):
                self.readback=readback
                self.assertTrue(self.send().success)
                self.assertEqual(self.state()[0],'accepted')

    def test_missing_or_empty_artifact_never_completes(self):
        for mode in ['empty','missing']:
            with self.subTest(mode=mode), self.assertLogs(level='ERROR'):
                if mode=='empty': self.plan.write_text('  ')
                else: self.plan.unlink()
                self.send()
                self.assertEqual(self.state()[0],'accepted')

    def test_no_action_silence_is_success_without_delivery(self):
        self.outcome.update(status='no_action',summary='Informational fixture only; no obligation or useful preparation required.')
        self.save_outcome(); self.plan.unlink()
        self.assertTrue(self.send('[SILENT]\nNo action required.').success)
        self.assertEqual(self.calls,[])
        self.assertEqual(self.state()[0],'completed')
        self.assertEqual(json.loads(Path(self.state()[1]).read_text())['notices'],[])

    def test_unsubstantiated_silence_quiet_but_not_completed(self):
        (self.folder/'outcome.json').unlink()
        with self.assertLogs(level='ERROR'): self.assertTrue(self.send('[SILENT]').success)
        self.assertEqual(self.calls,[])
        self.assertEqual(self.state()[0],'accepted')

    def test_real_dispatch_does_not_prepare_synthetic_checklist(self):
        event=NS(source=NS(platform=NS(value='webhook'),chat_id=self.chat),raw_message={'event_id':self.id,'life_prep_synthetic_prepare':True})
        self.assertIsNone(self.bridge.dispatch(event,self.gateway))
        self.assertFalse((self.folder/'checklist.md').exists())
        with sqlite3.connect(self.root/'state.db') as db: db.execute("update events set account='activation-synthetic'")
        self.assertEqual(self.bridge.dispatch(event,self.gateway)['action'],'skip')
        self.assertFalse((self.folder/'checklist.md').exists())

    def test_explicit_synthetic_probe_uses_separate_fixture(self):
        with sqlite3.connect(self.root/'state.db') as db: db.execute("update events set account='activation-synthetic'")
        shutil.rmtree(self.folder)
        self.bridge.unload()
        async def deliver(chat,content,**kw):
            self.content=content
            self.calls.append((chat,content))
            return SendResult(success=True,message_id='111')
        self.webhook.send=deliver
        event=NS(source=NS(platform=NS(value='webhook'),chat_id=self.chat),raw_message={'event_id':self.id,'life_prep_synthetic_prepare':True})
        result=asyncio.run(self.bridge.probe_dispatch(event,self.gateway))
        self.assertEqual(result['action'],'skip')
        self.assertEqual(self.state()[0],'completed')
        self.assertIn('Synthetic context validation', (self.folder/'checklist.md').read_text())
        self.assertEqual(len(self.calls),1)

    def test_missing_outcome_or_wrong_provenance_never_completes(self):
        self.outcome['source_ref']='wrong-source'
        self.save_outcome()
        with self.assertLogs(level='ERROR'): self.send()
        self.assertEqual(self.state()[0],'accepted')
        (self.folder/'outcome.json').unlink()
        with self.assertLogs(level='ERROR'): self.send()
        self.assertEqual(self.state()[0],'accepted')

    def test_old_receipt_is_not_useful_preparation(self):
        self.plan.unlink()
        (self.folder/'receipt.json').write_text('old unverified receipt')
        with self.assertLogs(level='ERROR'): self.send()
        self.assertEqual(self.state()[0],'accepted')

    def test_default_registration_disables_synthetic_probes(self):
        hooks={}
        config=self.root/'life-prep-context.json'
        config.write_text(json.dumps({'root':str(self.root),'discord_channel':'123'}))
        ctx=NS(register_hook=lambda name,fn:hooks.update({name:fn}),on_unload=lambda fn:None)
        with patch('hermes_constants.get_hermes_home',return_value=self.root):
            load().register(ctx)
        self.assertEqual(hooks['pre_gateway_dispatch'].__name__,'dispatch')
        config.write_text(json.dumps({'root':str(self.root),'discord_channel':'123','synthetic_probes':True}))
        with patch('hermes_constants.get_hermes_home',return_value=self.root):
            load().register(ctx)
        self.assertEqual(hooks['pre_gateway_dispatch'].__name__,'probe_dispatch')

    def test_receipt_command_failure_preserves_accepted(self):
        (self.root/'life-prep').unlink()
        with self.assertLogs(level='ERROR'): self.assertTrue(self.send().success)
        self.assertEqual(self.state()[0],'accepted')

    def test_concurrent_duplicate_is_one_delivery(self):
        async def run():
            return await asyncio.gather(self.webhook.send(self.chat,self.content),self.webhook.send(self.chat,self.content))
        self.assertTrue(all(r.success for r in asyncio.run(run())))
        self.assertEqual(len(self.calls),1)
        self.assertEqual(self.state()[0],'completed')

    def test_actual_webhook_adapter_send_seam(self):
        from gateway.config import PlatformConfig
        from gateway.platforms.webhook import WebhookAdapter
        adapter=WebhookAdapter(PlatformConfig(enabled=True,extra={}))
        adapter._delivery_info=self.webhook._delivery_info
        adapter.gateway_runner=self.gateway
        async def transport(deliver_type, content, delivery):
            self.calls.append((self.chat,content))
            return SendResult(success=True,message_id='111')
        adapter._deliver_cross_platform=transport
        self.gateway.adapters['webhook']=adapter
        self.bridge.install(self.gateway)
        self.assertTrue(asyncio.run(adapter.send(self.chat,self.content)).success)
        self.assertEqual(self.state()[0],'completed')
        self.assertEqual(len(self.calls),1)

    def test_other_route_unaffected(self):
        asyncio.run(self.webhook.send('webhook:other:x','[SILENT]'))
        self.assertEqual(self.calls,[('webhook:other:x','[SILENT]')])

if __name__=='__main__': unittest.main()
