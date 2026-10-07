#!/usr/bin/env python3
"""Snapshot private message storage and complete JSONL records; print only a manifest."""
import argparse, datetime, gzip, hashlib, json, os, sqlite3
from pathlib import Path

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--data',type=Path,default=Path('observer/private-data'))
p.add_argument('--out',type=Path,default=Path('observer/archives'))
a=p.parse_args()
os.umask(0o077)
now=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ')
out=a.out/now;out.mkdir(parents=True)
manifest={'started_utc':now,'files':{},'log_records':0,'source_log_bytes':0,'copied_log_bytes':0}
with sqlite3.connect(f'file:{(a.data/"cache.db").resolve()}?mode=ro',uri=True) as source, sqlite3.connect(out/'cache.db') as dest:
 source.backup(dest)
log=a.data/'requests.jsonl'
with log.open('rb') as source,gzip.open(out/'requests.jsonl.gz','wb') as dest:
 bound=os.fstat(source.fileno()).st_size
 manifest['source_log_bytes']=bound
 while source.tell()<bound:
  line=source.readline(bound-source.tell())
  if not line.endswith(b'\n'):break
  json.loads(line) # Refuse to silently archive malformed evidence.
  dest.write(line);manifest['log_records']+=1;manifest['copied_log_bytes']+=len(line)
for file in sorted(out.iterdir()):
 h=hashlib.sha256()
 with file.open('rb') as stream:
  for chunk in iter(lambda:stream.read(1024*1024),b''):h.update(chunk)
 manifest['files'][file.name]={'sha256':h.hexdigest(),'bytes':file.stat().st_size}
manifest['finished_utc']=datetime.datetime.now(datetime.timezone.utc).isoformat()
(out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'archive':str(out),'manifest':manifest},indent=2))
