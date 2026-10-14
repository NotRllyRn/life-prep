"""Verified delivery -> one anchored thread -> persistent mention-free admission."""
import asyncio
import os
import sys
from types import SimpleNamespace as NS
from unittest.mock import AsyncMock, patch
from test_context import ContextTests
from gateway.platforms.helpers import ThreadParticipationTracker


class NoticeThreadTests(ContextTests):
    def send_notice(self, *, existing=False, failure=False, mismatch=False, quiet=False, wrong_feed=False, unverified=False):
        event = 'a' * 64
        chat = 'webhook:life-prep:' + event
        summary = 'Useful\n' + 'summary ' * 30
        content = summary + event + ' ' + str(self.root / 'prepared' / event / 'plan.md')
        thread = NS(id=999 if mismatch else 111, parent_id=123)
        create = AsyncMock(return_value=thread, side_effect=RuntimeError('permission denied') if failure else None)
        messages = {111: NS(channel=NS(id=123), author=NS(id=99), content=content[:20], thread=thread if existing else None, create_thread=create),
                    222: NS(channel=NS(id=123), author=NS(id=99), content=content[20:])}
        if unverified:
            messages[111].author.id = 88
        channel = NS(fetch_message=AsyncMock(side_effect=lambda i: messages[i]))
        send = AsyncMock(return_value=NS(success=True, message_id='222', continuation_message_ids=('111',)))
        send._life_prep_owner = None
        webhook = NS(send=send, _delivery_info={chat: {'deliver': 'discord', 'deliver_extra': {'chat_id': '456' if wrong_feed else '123'}}})
        with patch.dict(os.environ, {'HERMES_HOME': str(self.root)}):
            tracker = ThreadParticipationTracker('discord')
            adapter = NS(_client=NS(user=NS(id=99), get_channel=lambda _: channel), _threads=tracker)
            gateway = NS(adapters={'webhook': webhook, 'discord': adapter})
            self.bridge.install(gateway)
            with patch.dict(sys.modules, {'gateway.mirror': NS(mirror_to_session=lambda *a, **k: None)}), patch.object(self.bridge, 'outcome', return_value={'summary': summary}), patch.object(self.bridge, 'receipt', new=AsyncMock()), patch.object(self.bridge, 'completed', return_value=False):
                result = asyncio.run(webhook.send(chat, '[SILENT]' if quiet else content))
            restored = ThreadParticipationTracker('discord')
        return result, create, restored, send, gateway

    def test_chunked_notice_creates_one_thread_and_persists(self):
        result, create, tracker, send, gateway = self.send_notice()
        self.assertTrue(result.success)
        create.assert_awaited_once()
        title = create.call_args.kwargs['name']
        self.assertLessEqual(len(title), 100)
        self.assertNotIn('\n', title)
        self.assertEqual(create.call_args.kwargs['auto_archive_duration'], 1440)
        self.assertIn('111', tracker)
        self.assertNotIn('222', tracker)
        import discord
        from plugins.platforms.discord.adapter import DiscordAdapter
        channel = object.__new__(discord.Thread)
        channel.id = 111
        admission = NS(_threads=tracker, _discord_thread_require_mention=lambda: False)
        self.assertTrue(DiscordAdapter._in_bot_thread(admission, NS(channel=channel)))
        admission._discord_thread_require_mention = lambda: True
        self.assertFalse(DiscordAdapter._in_bot_thread(admission, NS(channel=channel)))
        self.assertIn('email-A', self.bridge.dispatch(self.event(thread='111', parent='123', chat='111'), gateway)['text'])

    def test_existing_thread_is_reused(self):
        _, create, tracker, _, _ = self.send_notice(existing=True)
        create.assert_not_awaited()
        self.assertIn('111', tracker)

    def test_thread_failure_preserves_notice_without_resend(self):
        with self.assertLogs(self.mod.LOG, level='ERROR'):
            result, create, tracker, send, _ = self.send_notice(failure=True)
        self.assertTrue(result.success)
        send.assert_awaited_once()
        self.assertNotIn('111', tracker)
        self.assertIsNotNone(self.bridge.store.get('123', '111'))

    def test_wrong_thread_is_not_registered(self):
        with self.assertLogs(self.mod.LOG, level='ERROR'):
            _, _, tracker, _, _ = self.send_notice(mismatch=True)
        self.assertNotIn('999', tracker)
        self.assertNotIn('111', tracker)

    def test_quiet_and_other_feed_create_no_thread(self):
        for kwargs in ({'quiet': True}, {'wrong_feed': True}):
            with self.subTest(kwargs=kwargs):
                _, create, tracker, _, _ = self.send_notice(**kwargs)
                create.assert_not_awaited()
                self.assertNotIn('111', tracker)

    def test_unverified_notice_creates_no_thread(self):
        with self.assertLogs(self.mod.LOG, level='ERROR'):
            _, create, tracker, _, _ = self.send_notice(unverified=True)
        create.assert_not_awaited()
        self.assertNotIn('111', tracker)

    def test_completed_event_does_not_send_or_create(self):
        _, create, _, send, gateway = self.send_notice()
        with patch.object(self.bridge, 'completed', return_value=True):
            asyncio.run(gateway.adapters['webhook'].send('webhook:life-prep:' + 'a' * 64, 'duplicate'))
        create.assert_awaited_once()
        send.assert_awaited_once()
