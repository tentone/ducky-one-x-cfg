#!/usr/bin/env python3
"""Capture a web app's browser responses and serve the snapshot locally.

Install: pip install playwright && python -m playwright install chromium
Capture: python mirror_webapp.py download
Serve:   python mirror_webapp.py serve
"""
import argparse
import hashlib
import json
import mimetypes
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

TARGET = 'https://one-x.duckyhub.io/keyPress'
ROOT = Path(__file__).resolve().parent / 'webapp_snapshot'
PORT = 8765
TEXT_TYPES = ('text/', 'application/javascript', 'application/json', 'application/xml',
              'application/manifest+json', 'application/wasm-text')

def key_for(url):
    parts = urlsplit(url)
    return urlunsplit((parts.scheme.lower(), parts.netloc.lower(), parts.path or '/', parts.query, ''))

def save_response(response, directory, manifest):
    url = key_for(response.url)
    if response.status < 200 or response.status >= 400 or url in manifest:
        return
    try:
        body = response.body()
        headers = response.headers
    except Exception as exc:
        print(f'Skip {url}: {exc}')
        return
    digest = hashlib.sha256(url.encode()).hexdigest()
    extension = Path(urlsplit(url).path).suffix[:12]
    if not extension or not re.fullmatch(r'\.[a-zA-Z0-9]+', extension):
        extension = mimetypes.guess_extension(headers.get('content-type', '').split(';')[0]) or '.bin'
    name = digest + extension
    (directory / 'blobs' / name).write_bytes(body)
    manifest[url] = {'file': name, 'type': headers.get('content-type', 'application/octet-stream'),
                     'status': response.status}
    print(f'{response.status} {len(body):>9} {url}')

def download(target, directory, wait_seconds):
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        raise SystemExit('Install dependencies: pip install playwright && python -m playwright install chromium')
    (directory / 'blobs').mkdir(parents=True, exist_ok=True)
    manifest = {}
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(service_workers='block', viewport={'width': 1440, 'height': 900})
        page = context.new_page()
        page.on('response', lambda response: save_response(response, directory, manifest))
        print(f'Opening {target}')
        page.goto(target, wait_until='domcontentloaded', timeout=60000)
        page.wait_for_timeout(wait_seconds * 1000)
        try:
            page.evaluate('window.scrollTo(0, document.body.scrollHeight)')
            page.wait_for_timeout(1500)
            page.evaluate('window.scrollTo(0, 0)')
            page.wait_for_timeout(1000)
        except Exception:
            pass
        # Record the rendered DOM as a fallback for sites that render their HTML client-side.
        (directory / 'rendered.html').write_text(page.content(), encoding='utf-8')
        (directory / 'manifest.json').write_text(json.dumps({'target': target, 'resources': manifest}, indent=2), encoding='utf-8')
        browser.close()
    print(f'Captured {len(manifest)} resources in {directory}')
    print(f'Run: python {Path(__file__).name} serve')


def serve(directory, port):
    manifest_path = directory / 'manifest.json'
    if not manifest_path.exists():
        raise SystemExit('No snapshot found. Run the download command first.')
    data = json.loads(manifest_path.read_text(encoding='utf-8'))
    target = data['target']
    original_origin = f'{urlsplit(target).scheme}://{urlsplit(target).netloc}'
    resources = data['resources']
    # Map external captured URLs to same-origin local paths too.
    external_prefix = '/__external__/'

    def localize(url):
        parts = urlsplit(url)
        if f'{parts.scheme}://{parts.netloc}' == original_origin:
            return (parts.path or '/') + (('?' + parts.query) if parts.query else '')
        return external_prefix + parts.scheme + '/' + parts.netloc + (parts.path or '/') + (('?' + parts.query) if parts.query else '')

    lookup = {localize(url): entry for url, entry in resources.items()}
    replacements = [(url, localize(url)) for url in resources]
    replacements.sort(key=lambda item: len(item[0]), reverse=True)
    app_path = urlsplit(target).path

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requested = self.path.split('#', 1)[0]
            if requested == '/':
                self.send_response(302)
                self.send_header('Location', app_path)
                self.end_headers()
                return
            entry = lookup.get(requested)
            if not entry and requested.endswith('/'):
                entry = lookup.get(requested[:-1])
            if entry:
                body = (directory / 'blobs' / entry['file']).read_bytes()
                mime = entry['type']
                if mime.startswith(TEXT_TYPES) or mime.split(';')[0] in ('image/svg+xml',):
                    try:
                        content = body.decode('utf-8')
                        for remote, local in replacements:
                            content = content.replace(remote, local)
                        content = content.replace(original_origin, '')
                        body = content.encode('utf-8')
                    except UnicodeDecodeError:
                        pass
                self.send_response(200)
                self.send_header('Content-Type', mime)
                self.send_header('Content-Length', str(len(body)))
                self.send_header('Access-Control-Allow-Origin', '*')
                self.end_headers()
                self.wfile.write(body)
            elif requested == app_path and (directory / 'rendered.html').exists():
                body = (directory / 'rendered.html').read_bytes()
                self.send_response(200)
                self.send_header('Content-Type', 'text/html; charset=utf-8')
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            else:
                print(f'MISSING: {requested}')
                self.send_error(404, f'Not captured: {requested}')

    print(f'Open http://localhost:{port}{app_path}')
    ThreadingHTTPServer(('127.0.0.1', port), Handler).serve_forever()

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('command', choices=['download', 'serve'])
    parser.add_argument('--url', default=TARGET)
    parser.add_argument('--dir', type=Path, default=ROOT)
    parser.add_argument('--port', type=int, default=PORT)
    parser.add_argument('--wait', type=int, default=8, help='Seconds to allow dynamic requests to finish')
    args = parser.parse_args()
    if args.command == 'download':
        download(args.url, args.dir, args.wait)
    else:
        serve(args.dir, args.port)
