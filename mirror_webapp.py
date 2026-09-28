#!/usr/bin/env python3
"""Capture the five ONE-X app routes and serve their browser-visible resources locally.

Install: python -m pip install playwright && python -m playwright install chromium
Capture: python mirror_webapp.py download
Serve:   python mirror_webapp.py serve

A browser snapshot cannot reproduce remote backend behavior or hardware integrations.
"""
import argparse
import hashlib
import json
import mimetypes
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

ORIGIN = 'https://one-x.duckyhub.io'
ROUTES = ('/keyPress', '/light', '/stroke', '/trigger', '/macro')
ROOT = Path(__file__).resolve().parent / 'webapp_snapshot'
PORT = 8765
TEXT_TYPES = ('text/', 'application/javascript', 'application/json', 'application/xml',
              'application/manifest+json', 'application/wasm-text', 'image/svg+xml')


def normalized(url):
    p = urlsplit(url)
    return urlunsplit((p.scheme.lower(), p.netloc.lower(), p.path or '/', p.query, ''))


def save_response(response, directory, resources):
    url = normalized(response.url)
    if not (200 <= response.status < 400):
        return
    try:
        body = response.body()
        mime = response.headers.get('content-type', 'application/octet-stream')
    except Exception as exc:
        print(f'SKIP {url}: {exc}')
        return
    digest = hashlib.sha256(url.encode('utf-8')).hexdigest()
    suffix = Path(urlsplit(url).path).suffix[:12]
    if not re.fullmatch(r'\.[A-Za-z0-9]+', suffix):
        suffix = mimetypes.guess_extension(mime.split(';')[0]) or '.bin'
    filename = digest + suffix
    (directory / 'blobs' / filename).write_bytes(body)
    resources[url] = {'file': filename, 'type': mime, 'status': response.status}
    print(f'{response.status} {len(body):>9} {url}')


def download(origin, routes, directory, wait_seconds):
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        raise SystemExit('Install: python -m pip install playwright && python -m playwright install chromium')

    (directory / 'blobs').mkdir(parents=True, exist_ok=True)
    manifest_path = directory / 'manifest.json'
    # Keep previous captures, allowing incremental downloads into the same snapshot.
    old = json.loads(manifest_path.read_text('utf-8')) if manifest_path.exists() else {}
    if old.get('origin') and old['origin'] != origin:
        raise SystemExit(f'Snapshot belongs to {old["origin"]}; use a different --dir.')
    resources = old.get('resources', {})
    pages = old.get('pages', {})
    errors = []

    def persist():
        manifest_path.write_text(json.dumps({
            'origin': origin, 'routes': list(dict.fromkeys([*old.get('routes', []), *routes])),
            'resources': resources, 'pages': pages,
        }, indent=2), encoding='utf-8')

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(service_workers='block', viewport={'width': 1440, 'height': 900})
        # Capture from every page; route-specific lazy assets can be loaded by scrolling.
        for route in routes:
            page = context.new_page()
            page.on('response', lambda response: save_response(response, directory, resources))
            url = origin + route
            print(f'\nOpening {url}')
            try:
                response = page.goto(url, wait_until='domcontentloaded', timeout=60000)
                if response is None or response.status >= 400:
                    raise RuntimeError(f'Navigation HTTP status: {response.status if response else "none"}')
                page.wait_for_timeout(wait_seconds * 1000)
                page.evaluate('window.scrollTo(0, document.body.scrollHeight)')
                page.wait_for_timeout(1200)
                page.evaluate('window.scrollTo(0, 0)')
                page.wait_for_timeout(500)
                # The captured document is a per-route fallback if the website uses SPA navigation.
                html = page.content()
                filename = hashlib.sha256(route.encode('utf-8')).hexdigest() + '.html'
                (directory / 'blobs' / filename).write_text(html, encoding='utf-8')
                pages[route] = filename
                print(f'Saved rendered page: {route}')
            except Exception as exc:
                errors.append((route, str(exc)))
                print(f'ERROR {route}: {exc}')
            finally:
                persist()
                page.close()
        browser.close()

    print(f'\nCaptured {len(resources)} resources and {len(pages)} pages in {directory}')
    if errors:
        print('Failed routes (retry download): ' + ', '.join(route for route, _ in errors))
    print(f'Run: python {Path(__file__).name} serve')


def serve(directory, port):
    path = directory / 'manifest.json'
    if not path.exists():
        raise SystemExit('No snapshot found. Run download first.')
    data = json.loads(path.read_text(encoding='utf-8'))
    origin = data.get('origin') or ('https://' + urlsplit(data['target']).netloc)
    resources = data['resources']
    pages = data.get('pages', {})
    if not pages and (directory / 'rendered.html').exists():
        # Compatibility with snapshots from the original script.
        pages[urlsplit(data['target']).path] = 'rendered.html'
    routes = data.get('routes', list(pages))
    external_prefix = '/__external__/'

    def localize(url):
        p = urlsplit(url)
        if f'{p.scheme}://{p.netloc}' == origin:
            return (p.path or '/') + ('?' + p.query if p.query else '')
        return f'{external_prefix}{p.scheme}/{p.netloc}{p.path or "/"}' + ('?' + p.query if p.query else '')

    lookup = {localize(url): entry for url, entry in resources.items()}
    replacements = sorted(((url, localize(url)) for url in resources), key=lambda x: len(x[0]), reverse=True)

    def rewrite(body, mime):
        if not mime.lower().startswith(TEXT_TYPES):
            return body
        try:
            text = body.decode('utf-8')
        except UnicodeDecodeError:
            return body
        for remote, local in replacements:
            text = text.replace(remote, local)
        text = text.replace(origin, '')
        return text.encode('utf-8')

    class Handler(BaseHTTPRequestHandler):
        def send_body(self, body, mime):
            self.send_response(200)
            self.send_header('Content-Type', mime)
            self.send_header('Content-Length', str(len(body)))
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()
            if self.command != 'HEAD':
                self.wfile.write(body)

        def do_HEAD(self):
            self.do_GET()

        def do_GET(self):
            requested = self.path.split('#', 1)[0]
            pathname = urlsplit(requested).path
            if requested == '/':
                self.send_response(302)
                self.send_header('Location', routes[0] if routes else '/keyPress')
                self.end_headers()
                return
            # Prefer the recorded network response, and use per-route DOM fallback.
            entry = lookup.get(requested) or lookup.get(requested.rstrip('/'))
            if entry:
                body = (directory / 'blobs' / entry['file']).read_bytes()
                self.send_body(rewrite(body, entry['type']), entry['type'])
                return
            filename = pages.get(pathname.rstrip('/'))
            if filename:
                candidate = directory / 'blobs' / filename
                if not candidate.exists():
                    candidate = directory / filename  # Original-script compatibility.
                self.send_body(rewrite(candidate.read_bytes(), 'text/html'), 'text/html; charset=utf-8')
                return
            print(f'MISSING: {requested}')
            self.send_error(404, f'Not captured: {requested}')

    print(f'Serving {len(pages)} captured routes at http://localhost:{port}/')
    for route in routes:
        print(f'  http://localhost:{port}{route}')
    ThreadingHTTPServer(('127.0.0.1', port), Handler).serve_forever()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('command', choices=['download', 'serve'])
    parser.add_argument('--origin', default=ORIGIN, help='Remote website origin')
    parser.add_argument('--routes', nargs='+', default=list(ROUTES), help='Paths to capture')
    parser.add_argument('--dir', type=Path, default=ROOT)
    parser.add_argument('--port', type=int, default=PORT)
    parser.add_argument('--wait', type=int, default=8, help='Seconds to wait on each page')
    args = parser.parse_args()
    if args.command == 'download':
        download(args.origin.rstrip('/'), [r if r.startswith('/') else '/' + r for r in args.routes], args.dir, args.wait)
    else:
        serve(args.dir, args.port)
