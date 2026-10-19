"""Reproduce la CLI base con yt-dlp falso; no prueba descarga remota."""
import os, pathlib as P, tempfile, subprocess as S, json, sqlite3
root=P.Path(tempfile.mkdtemp(prefix='mu7base-'));dest=P.Path(__file__).resolve().parent;results=[]
for case in ['new','existing','empty','failure','partial']:
 r=root/case
 for d in ['cfg/maly','data','run','music','bin']:(r/d).mkdir(parents=True)
 (r/'cfg/maly/config.toml').write_text(f'language="es"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\n')
 (r/'bin/yt-dlp').write_text('''#!/usr/bin/python3
import sys,pathlib as P,os
args=sys.argv[1:];p=P.Path(os.environ['OUT']);report=args[args.index('--print-to-file')+2]
mode=os.environ['CASE']
if mode in ('new','partial'):p.write_bytes(b'audio simulado')
if mode in ('new','existing','partial'):P.Path(report).write_text(str(p)+'\\n')
if mode in ('failure','partial'):sys.exit(1)
''');(r/'bin/yt-dlp').chmod(0o755)
 if case=='existing':(r/'music/Igual.mp3').write_bytes(b'existente sin indexar')
 env=os.environ|{'XDG_CONFIG_HOME':str(r/'cfg'),'XDG_DATA_HOME':str(r/'data'),'XDG_RUNTIME_DIR':str(r/'run'),'PATH':str(r/'bin')+':'+os.environ['PATH'],'CASE':case,'OUT':str(r/'music/Igual.mp3')}
 p=S.run(['/tmp/maly-ux07-before','get','https://fixture.invalid/audio'],env=env,text=True,capture_output=True)
 db=r/'data/maly/library.db';indexed=[]
 if db.exists():
  with sqlite3.connect(db) as c:indexed=[row[0] for row in c.execute('SELECT path FROM tracks')]
 results.append({'case':case,'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr,'files':[x.name for x in (r/'music').iterdir()],'indexed':indexed})
assert results[0]['indexed'] and not results[1]['indexed'] and results[4]['files'] and not results[4]['indexed']
(dest/'baseline.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print('Reproducido: nuevo indexado; existente sin indexar; parcial con audio sin indexar.')
