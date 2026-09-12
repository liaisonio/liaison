"""Loopback-only protocol fixture. Not an AI model, never returns real inference."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import time

class Handler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    def log_message(self, *args):
        pass
    def reply(self, status, value):
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)
    def do_GET(self):
        if self.path == '/v1/models':
            return self.reply(200, {'object':'list','data':[{'id':'fixture-chat'}]})
        if self.path == '/anthropic/models':
            return self.reply(200, {'data':[{'id':'fixture-chat','type':'model'}], 'has_more':False})
        return self.reply(404, {})
    def do_POST(self):
        try:
            self.handle_post()
        except (BrokenPipeError, ConnectionResetError):
            pass
    def handle_post(self):
        length = int(self.headers.get('Content-Length', '0'))
        if not 0 < length <= 1048576:
            return self.reply(400, {})
        body = json.loads(self.rfile.read(length))
        if self.path not in ('/v1/chat/completions','/anthropic/messages'):
            return self.reply(404,{})
        if body.get('model') != 'fixture-chat':
            return self.reply(400, {'error':'wrong model mapping'})
        anthropic = self.path.startswith('/anthropic')
        if anthropic and self.headers.get('anthropic-version') != '2023-06-01':
            return self.reply(400, {})
        text = 'Protocol fixture: connector forwarding verified. This is not model inference.'
        command = body.get('messages',[{}])[-1].get('content')
        if not anthropic and command == 'quota' and body.get('stream') and not body.get('stream_options',{}).get('include_usage'):
            return self.reply(400, {'error':'limited stream must request usage'})
        if command == 'missing-usage':
            if anthropic:
                return self.reply(200, {'id':'fixture-1','content':[{'type':'text','text':text}], 'stop_reason':'end_turn'})
            return self.reply(200, {'id':'fixture-1','model':'fixture-chat','choices':[{'index':0,'message':{'role':'assistant','content':text},'finish_reason':'stop'}]})
        if not body.get('stream'):
            if anthropic:
                return self.reply(200, {'id':'fixture-1','content':[{'type':'text','text':text}], 'stop_reason':'end_turn','usage':{'input_tokens':7,'output_tokens':9}})
            return self.reply(200, {'id':'fixture-1','object':'chat.completion','model':'fixture-chat','choices':[{'index':0,'message':{'role':'assistant','content':text},'finish_reason':'stop'}],'usage':{'prompt_tokens':7,'completion_tokens':9}})
        self.send_response(200)
        self.send_header('Content-Type','text/event-stream')
        self.send_header('Connection','close')
        self.end_headers()
        def event(value):
            encoded = value if isinstance(value,str) else json.dumps(value)
            self.wfile.write(('data: '+encoded+'\n\n').encode());self.wfile.flush()
        slow = body.get('messages',[{}])[-1].get('content') == 'slow-fixture'
        if anthropic:
            event({'type':'message_start','message':{'id':'fixture-1','usage':{'input_tokens':7,'output_tokens':0}}})
            event({'type':'content_block_start','index':0,'content_block':{'type':'text','text':''}})
            event({'type':'content_block_delta','index':0,'delta':{'type':'text_delta','text':text}})
            event({'type':'message_delta','delta':{'stop_reason':'end_turn'},'usage':{'output_tokens':9}})
            event({'type':'message_stop'})
        else:
            event({'id':'fixture-1','object':'chat.completion.chunk','model':'fixture-chat','choices':[{'index':0,'delta':{'content':text},'finish_reason':None}]})
            if slow:
                for _ in range(12):
                    time.sleep(1)
                    self.wfile.write(b': heartbeat\n\n');self.wfile.flush()
            event({'id':'fixture-1','model':'fixture-chat','choices':[{'index':0,'delta':{},'finish_reason':'stop'}],'usage':{'prompt_tokens':7,'completion_tokens':9}})
            event('[DONE]')
        self.close_connection = True

ThreadingHTTPServer(('127.0.0.1',19380), Handler).serve_forever()
