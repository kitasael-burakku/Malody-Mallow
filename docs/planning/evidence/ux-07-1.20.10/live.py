"""dbus-run-session -- python3 live.py /tmp/maly-ux07-after
TUI real, mpv ao=null y yt-dlp SIMULADO. No valida el servicio remoto.
"""
import os,pathlib as P,subprocess as S,tempfile,time,json,socket,sqlite3,shlex,sys,signal
binary=str(P.Path(sys.argv[1]).resolve());dest=P.Path(__file__).resolve().parent
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path(tempfile.mkdtemp(prefix='mu7-'));results=[]
for lang,profile,color in [('es','default',True),('en','vim',False),('es','override',False)]:
 r=root/profile
 for d in ['cfg/maly','cfg/mpv','data','run','music','bin']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\nget="ctrl+y"\n'
 (r/'cfg/maly/config.toml').write_text(cfg);(r/'cfg/mpv/mpv.conf').write_text('ao=null\n')
 S.run(['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-t','120','-c:a','libmp3lame','-metadata','title=La misma canción 東京','-metadata','artist=Áurea','-y',str(r/'sample.mp3')],check=True)
 (r/'bin/yt-dlp').write_text('''#!/usr/bin/python3
import sys,os,pathlib as P,json,time,subprocess
r=P.Path(os.environ['FIXTURE']);args=sys.argv[1:];state=json.loads((r/'state.json').read_text())
(r/'pid').write_text(str(os.getpid()))
if state['mode']=='slow':
 print('DESCARGA SIMULADA EN CURSO',flush=True)
 time.sleep(120)
 sys.exit(0)
if '--dump-json' in args:
 for i in range(2):print(json.dumps({'title':'La misma canción 東京','uploader':'Áurea','duration':120,'url':'https://fixture.invalid/'+str(i)}))
 sys.exit(0)
out=args[args.index('-o')+1];directory=P.Path(out).parent
if '%(playlist_title)s' in str(directory):directory=P.Path(str(directory).replace('%(playlist_title)s','Lista remota'))
directory.mkdir(parents=True,exist_ok=True)
paths=[]
if state['mode'] not in ('empty','failure'):
 for name in state['names']:
  p=directory/name
  if not p.exists():p.write_bytes((r/'sample.mp3').read_bytes())
  paths.append(str(p))
if '--print-to-file' in args:P.Path(args[args.index('--print-to-file')+2]).write_text(''.join(x+'\\n' for x in paths))
if state['mode'] in ('failure','partial'):
 print('FALLO SIMULADO',file=sys.stderr)
 sys.exit(1)
''');(r/'bin/yt-dlp').chmod(0o755)
 env=os.environ|{'XDG_CONFIG_HOME':str(r/'cfg'),'XDG_DATA_HOME':str(r/'data'),'XDG_RUNTIME_DIR':str(r/'run'),'XDG_CACHE_HOME':str(r/'cache'),'FIXTURE':str(r),'PATH':str(r/'bin')+':'+os.environ['PATH'],'TERM':'xterm-256color'};env.pop('NO_COLOR',None)
 if not color:env['NO_COLOR']='1'
 def state(mode,names): (r/'state.json').write_text(json.dumps({'mode':mode,'names':names}))
 state('new',['01.mp3','02.mp3'])
 target='ux:main';tmargs=['tmux','-S',str(r/'tmux')]
 def tm(*args):return S.check_output(tmargs+list(args),env=env,text=True,stderr=S.STDOUT)
 def key(*args):tm('send-keys','-t',target,*args)
 def type_text(text):
  S.run(tmargs+['load-buffer','-'],env=env,input=text,text=True,check=True);tm('paste-buffer','-d','-p','-t',target)
 def capture():return tm('capture-pane','-p','-t',target)
 def until(fn):
  deadline=time.monotonic()+15
  while time.monotonic()<deadline:
   try:
    value=fn()
    if value:return value
   except (FileNotFoundError,ConnectionRefusedError):pass
   time.sleep(.05)
  raise AssertionError('Estado no llegó: '+capture())
 def wait(*words):
  def check():
   out=capture();flat=' '.join(out.replace('│',' ').split());return out if all(x in flat for x in words) else None
  return until(check)
 def save(name):
  (dest/f'{profile}-{name}.txt').write_text(capture());(dest/f'{profile}-{name}.ansi').write_text(tm('capture-pane','-p','-e','-t',target))
 def cli(*args,code=0):
  p=S.run([binary,*args],env=env,capture_output=True,text=True);assert p.returncode==code,(args,p.stdout,p.stderr);return p.stdout+p.stderr
 def ipc(cmd):
  with socket.socket(socket.AF_UNIX) as c:
   c.settimeout(12);c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd})+'\n').encode());return json.loads(c.makefile().readline())
 def queue():return [t['path'] for t in ipc('queue').get('queue',[])]
 def indexed(name):
  with sqlite3.connect(r/'data/maly/library.db') as db:return db.execute('SELECT count(*) FROM tracks WHERE path=?',(str(r/'music'/name),)).fetchone()[0]==1
 def console(command):
  key('C-p');wait('Paleta' if lang=='es' else 'Palette');type_text(command);key('Enter')
 def alive(pid):
  try:os.kill(pid,0);return True
  except ProcessLookupError:return False
 daemon=None
 if profile=='override':daemon=S.Popen([binary,'daemon'],env=env,stdout=(r/'daemon.log').open('w'),stderr=S.STDOUT)
 # El shell sigue vivo cuando sale maly: tmux no puede ocultar un hijo huérfano.
 launch='echo $$ > '+shlex.quote(str(r/'shell.pid'))+'; '+shlex.quote(binary)+'; echo MALY_TERMINO; sleep 60'
 tm('new-session','-d','-s','ux','-n','main','-x','140','-y','42',shlex.join(['/bin/bash','-c',launch]))
 getkey='C-y' if profile=='override' else 'C-g'
 try:
  until(lambda:(r/'run/maly/maly.sock').exists());wait('Biblioteca' if lang=='es' else 'Library')
  # Selección remota -> archivos locales, sin reproducción automática.
  key(getkey);type_text('canción');key('Enter');wait('enter descarga' if lang=='es' else 'enter downloads');save('search')
  key('Enter');until(lambda:indexed('02.mp3'));wait('01.mp3','02.mp3','tab');assert queue()==[];save('new')
  for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
   tm('resize-window','-t',target,'-x',str(w),'-y',str(h));wait('enter','tab','esc');save(f'{w}x{h}-files')
   assert queue()==[];results.append({'profile':profile,'language':lang,'color':color,'size':[w,h],'no_autoplay':True})
  key('Down','Tab');until(lambda:queue()==[str(r/'music/02.mp3')]);key('Enter');until(lambda:ipc('status')['status'].get('track',{}).get('path')==str(r/'music/02.mp3'));cli('pause')
  # La segunda grabación comparte metadatos: la ruta, no el título, manda.
  before=queue();state('existing',['existente con un título muy largo 東京 y espacios.mp3']);(r/'music/existente con un título muy largo 東京 y espacios.mp3').write_bytes((r/'sample.mp3').read_bytes());assert not indexed('existente con un título muy largo 東京 y espacios.mp3')
  console('get https://fixture.invalid/existing');until(lambda:indexed('existente con un título muy largo 東京 y espacios.mp3'));wait('Ya estaba:' if lang=='es' else 'Already present:');save('existing');assert queue()==before
  tm('resize-window','-t',target,'-x','40','-y','12');wait('Ya estaba:' if lang=='es' else 'Already present:');save('existing-small');tm('resize-window','-t',target,'-x','140','-y','42');key('Escape')
  state('partial',['parcial.mp3']);console('get https://fixture.invalid/partial');until(lambda:indexed('parcial.mp3'));wait('parcial' if lang=='es' else 'Partial');save('partial');assert queue()==before;key('Escape')
  for mode in ['empty','failure']:
   state(mode,[]);console('get https://fixture.invalid/'+mode);wait('no dejó' if mode=='empty' and lang=='es' else 'left no' if mode=='empty' else 'yt-dlp');save(mode);assert queue()==before;key('Escape')
  # SQLite ocupado: descarga conservada, acciones bloqueadas hasta reintento.
  state('new',['recuperar.mp3'])
  db=sqlite3.connect(r/'data/maly/library.db');db.execute('BEGIN IMMEDIATE')
  console('get https://fixture.invalid/scan-error');wait('ctrl+r');save('scan-failed');key('Enter','Tab');assert queue()==before;db.rollback();db.close()
  tm('resize-window','-t',target,'-x','40','-y','12');wait('ctrl+r','esc');save('scan-failed-small')
  key('C-r');until(lambda:indexed('recuperar.mp3'));wait('tab');save('scan-retried');assert queue()==before;key('Escape');tm('resize-window','-t',target,'-x','140','-y','42')
  # La ruta playlist conserva su flujo, y ofrece los archivos tras crearse.
  state('partial',['01 - tema.mp3','02 - tema.mp3']);console('get playlist https://fixture.invalid/list Lista')
  until(lambda:'Lista' in cli('playlist','list'));wait('01 - tema.mp3','02 - tema.mp3');save('playlist-partial');assert queue()==before;key('Escape')
  # El marcador de propiedad nunca certifica la grabación.
  state('new',['01.mp3']);key(getkey);type_text('canción');key('Enter');wait('✓','no identidad' if lang=='es' else 'not identity');save('owned-hint');key('Escape')
  # Cancelar búsqueda mata su proceso; no se cierra el pane.
  state('slow',[]);(r/'pid').unlink(missing_ok=True);key(getkey);type_text('lenta');key('Enter');until(lambda:'DESCARGA' not in capture() and (r/'pid').exists())
  until(lambda:(r/'pid').exists());pid=int((r/'pid').read_text());key('Escape');until(lambda:not alive(pid));save('search-cancelled')
  # Cancelar la descarga en terminal devuelve un error visible y no deja hijo.
  console('get https://fixture.invalid/slow');wait('DESCARGA SIMULADA EN CURSO');pid=int((r/'pid').read_text());key('C-c');until(lambda:not alive(pid));wait('yt-dlp');save('download-cancelled')
  # Cerrar maly durante búsqueda: el shell permanece para detectar huérfanos.
  key('Escape');state('slow',[]);(r/'pid').unlink(missing_ok=True)
  key(getkey);type_text('cerrar');key('Enter');until(lambda:(r/'pid').exists());pid=int((r/'pid').read_text())
  key('C-c');wait('MALY_TERMINO');until(lambda:not alive(pid));save('exit-search')
  results.append({'exit_during_search_no_child':True,'profile':profile,'exact_second_path':True,'existing_indexed':True,'partial_indexed':True,'failed_scan_retry':True,'playlist_continuation':True,'search_and_download_cancelled_no_child':True})
 finally:
  if 'MALY_TERMINO' not in capture():key('C-c')
  if daemon:daemon.terminate();daemon.wait(timeout=12)
  until(lambda:not (r/'run/maly/maly.sock').exists())
  tm('kill-session','-t','ux')
 (dest/'matrix.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(profile,'OK',flush=True)
