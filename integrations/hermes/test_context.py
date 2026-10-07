from contextlib import closing
import asyncio
import importlib.util
import json
from pathlib import Path
import sqlite3
import tempfile
from types import SimpleNamespace as NS
import unittest

PLUGIN = Path(__file__).parent / 'life-prep-context' / '__init__.py'
def load():
    spec = importlib.util.spec_from_file_location('life_prep_context_test', PLUGIN)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

class ContextTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.mod = load()
        self.bridge = self.mod.Bridge(self.root, '123')
        (self.root / 'artifacts').mkdir()
        with closing(sqlite3.connect(self.root/'state.db')) as db, db:
            db.execute('CREATE TABLE events(id,account,body,artifact,result)')
            for event, source in [('a'*64, 'email-A'), ('b'*64, 'email-B')]:
                artifact = self.root/'artifacts'/(event+'.json')
                artifact.write_text('{}')
                db.execute('INSERT INTO events VALUES(?,?,?,?,?)',(event,'synthetic',json.dumps({'ID':source}),str(artifact),''))
                prepared = self.root/'prepared'/event
                prepared.mkdir(parents=True)
                (prepared/'plan.md').write_text('Real fixture plan '+source)
        self.gateway = NS(adapters={})

    def event(self, pointer=None, thread=None, parent=None, chat='123'):
        source=NS(platform=NS(value='discord'),chat_id=chat,thread_id=thread,parent_chat_id=parent)
        return NS(source=source,text='expand / brief',reply_to_message_id=pointer,raw_message=None)

    def test_two_records_survive_module_and_store_recreation(self):
        for message,event in [('111','a'*64),('222','b'*64)]:
            context=self.bridge.prepared_context(event,'webhook:life-prep:'+event)
            self.bridge.store.put('123',message,context)
        bridge=load().Bridge(self.root,'123')
        reply=bridge.dispatch(self.event(pointer='111'),self.gateway)['text']
        thread=bridge.dispatch(self.event(thread='222',parent='123',chat='222'),self.gateway)['text']
        self.assertIn('email-A',reply)
        self.assertNotIn('email-B',reply)
        self.assertIn('email-B',thread)
        self.assertNotIn('email-A',thread)
        for event in [self.event(pointer='unknown'), self.event(thread='unknown',parent='123',chat='unknown'),self.event()]:
            result=bridge.dispatch(event,self.gateway)['text']
            self.assertIn('Do not use latest',result)
            self.assertNotIn('email-A',result)
            self.assertNotIn('email-B',result)

    def test_scope_and_collision(self):
        self.assertIsNone(self.bridge.dispatch(self.event(pointer='111',chat='456'),self.gateway))
        self.bridge.store.put('123','111',{'event_id':'a'*64})
        with self.assertRaises(ValueError): self.bridge.store.put('123','111',{'event_id':'b'*64})

    def test_paths_and_missing_preparation_fail_closed(self):
        with self.assertRaises(ValueError):self.bridge.prepared_context('../bad','x')
        context=self.bridge.prepared_context('a'*64,'x')
        self.bridge.store.put('123','111',context)
        (self.root/'prepared'/('a'*64)/'plan.md').unlink()
        self.assertIn('Fail closed',self.bridge.dispatch(self.event(pointer='111'),self.gateway)['text'])
        outside=self.root/'outside.md';outside.write_text('not approved')
        (self.root/'prepared'/('a'*64)/'escape.md').symlink_to(outside)
        with self.assertRaises(ValueError):self.bridge.prepared_context('a'*64,'x')

    def test_verified_delivery_maps_all_chunks_and_session(self):
        import sys
        import threading
        from unittest.mock import patch
        async def original(*args, **kwargs): return NS(success=True, message_id='333', continuation_message_ids=('111','222'))
        calls=[]
        async def fetch(message_id):
            calls.append(message_id)
            return NS(channel=NS(id=123),author=NS(id=99),content='Prepared fixture')
        channel=NS(fetch_message=fetch)
        client=NS(user=NS(id=99),get_channel=lambda _:channel)
        discord=NS(_client=client)
        event='a'*64
        chat='webhook:life-prep:'+event
        webhook=NS(send=original,_delivery_info={chat:{'deliver':'discord','deliver_extra':{'chat_id':'123'}}})
        entry=NS(origin=NS(chat_id=chat),session_id='actual-session',session_key='actual-key')
        gateway=NS(adapters={'webhook':webhook,'discord':discord},session_store=NS(_lock=threading.Lock(),_entries={'k':entry}))
        self.bridge.install(gateway)
        mirrors=[]
        fake=NS(mirror_to_session=lambda *args,**kwargs:mirrors.append(args))
        with patch.dict(sys.modules,{'gateway':NS(),'gateway.mirror':fake}):
            asyncio.run(webhook.send(chat,'Generated notice'))
        self.assertEqual(calls,[111,222,333])
        for pointer in ['111','222','333']:
            saved=self.bridge.store.get('123',pointer)
            self.assertEqual(saved['event_id'],event)
            self.assertEqual(saved['originating_session_id'],'actual-session')
        self.assertEqual(len(mirrors),1)
        self.bridge.unload()

    def test_delivered_mapping_failure_does_not_resend(self):
        calls=[]
        async def original(*args,**kwargs):
            calls.append(args)
            return NS(success=True,message_id='111')
        chat='webhook:life-prep:'+('c'*64)
        adapter=NS(send=original,_delivery_info={chat:{'deliver':'discord','deliver_extra':{'chat_id':'123'}}})
        self.bridge.install(NS(adapters={'webhook':adapter}))
        with self.assertLogs(self.mod.LOG,level='ERROR'):
            result=asyncio.run(adapter.send(chat,'Already delivered'))
        self.assertTrue(result.success)
        self.assertEqual(len(calls),1)
        self.assertIsNone(self.bridge.store.get('123','111'))

    def test_instance_wrapper_unloads_without_restart(self):
        async def original(*args,**kwargs): return NS(success=True,message_id=None)
        adapter=NS(send=original)
        gateway=NS(adapters={'webhook':adapter})
        self.bridge.install(gateway)
        wrapper=adapter.send
        self.bridge.install(gateway)
        self.assertIs(adapter.send,wrapper)
        asyncio.run(adapter.send('other','test'))
        self.bridge.unload()
        self.assertIs(adapter.send,original)

if __name__=='__main__':unittest.main()
