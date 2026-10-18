"""Ejecutar: dbus-run-session -- python3 tmux-matrix.py BINARIO SALIDA before|after|actions.
Genera datos propios; capturas de buffer tmux, no del escritorio.
"""
import os, subprocess as S, pathlib as P, time, json, sys, tempfile, shlex, socket
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
binary=str(P.Path(sys.argv[1]).resolve()); dest=P.Path(sys.argv[2]).resolve(); dest.mkdir(parents=True,exist_ok=True)
phase=sys.argv[3]; root=P.Path(tempfile.mkdtemp(prefix='mu4-')); print(root,flush=True); results=[]
for lang,profile,color in ([('es','default',True)] if phase in ('before','actions') else [('es','default',True),('en','vim',False),('es','override',False)]):
 r=root/f'{lang}-{profile}'
 for d in ['config/maly','config/mpv','data','run','music']:(r/d).mkdir(parents=True)
 os.chmod(r/'run',0o700)
 cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\ncontrols="{"vim" if profile=="vim" else "default"}"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
 if profile=='override':cfg+='[keys]\nsongs="ctrl+y"\n'
 (r/'config/maly/config.toml').write_text(cfg);(r/'config/mpv/mpv.conf').write_text('ao=null\n')
 e=os.environ.copy();e.update(XDG_CONFIG_HOME=str(r/'config'),XDG_DATA_HOME=str(r/'data'),XDG_RUNTIME_DIR=str(r/'run'),XDG_CACHE_HOME=str(r/'cache'),TERM='xterm-256color');e.pop('NO_COLOR',None)
 if not color:e['NO_COLOR']='1'
 tracks=[('Has Mo','Áurea','Horizontes','01'),('hsm','Aure\u0301a','Horizontes','02'),('Live','Áurea','','03'),('夜に Café','Áurea','','04'),('Áurea','Otra','Live','05'),('','','','Cancio\u0301n 東京')]
 paths=[]
 for title,artist,album,filename in tracks:
  path=r/f'music/{filename}.flac';paths.append(str(path))
  args=['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono','-t','120','-c:a','flac']
  for key,value in [('title',title),('artist',artist),('album',album)]:
   if value:args+=['-metadata',f'{key}={value}']
  S.run(args+['-y',str(path)],check=True)
 def cli(*args):return S.check_output([binary,*args],env=e,text=True,stderr=S.STDOUT)
 def tm(*args):return S.check_output(['tmux','-S',str(r/'tmux'),*args],env=e,text=True,stderr=S.STDOUT)
 target='ux:main'
 def key(*args):tm('send-keys','-t',target,*args)
 def cap():return tm('capture-pane','-p','-t',target)
 def wait(*want,absent=()):
  deadline=time.monotonic()+15
  while time.monotonic()<deadline:
   out=cap(); visible=' '.join(out.replace('│',' ').split())
   if all(s in visible for s in want) and all(s not in out for s in absent):return out
   time.sleep(.05)
  raise AssertionError((target,want,absent,out))
 def query(q,n):
  key('End',*(['BSpace']*100));key('-l',q)
  wait(q,f'{n} resultado(s)' if lang=='es' else f'{n} result(s)')
 def save(name):
  prefix=f'{phase}-{lang}-{profile}-{name}'
  (dest/f'{prefix}.txt').write_text(cap());(dest/f'{prefix}.ansi').write_text(tm('capture-pane','-p','-e','-t',target))
 def ipc(cmd):
  with socket.socket(socket.AF_UNIX) as c:
   c.connect(str(r/'run/maly/maly.sock'));c.sendall((json.dumps({'cmd':cmd})+'\n').encode());return json.loads(c.makefile().readline())
 def until(fn):
  deadline=time.monotonic()+10
  while time.monotonic()<deadline:
   if fn():return
   time.sleep(.05)
  raise AssertionError('Estado IPC no llegó')
 tm('new-session','-d','-s','ux','-n','main','-x','140','-y','42',shlex.quote(binary))
 try:
  wait('maly scan');cli('scan');wait('Áurea')
  key('/');key('-l','a:Áurea');key('Enter');wait('/a:Áurea');save('slash');key('Escape')
  for surface in ['songs','select']:
   if surface=='songs':key('C-y' if profile=='override' else 'C-o')
   else:
    tm('new-window','-d','-t','ux','-n','select',shlex.quote(binary)+' select; exec bash --noprofile --norc');target='ux:select'
   wait(('Canciones' if lang=='es' else 'Songs') if surface=='songs' else ('Elegir canción' if lang=='es' else 'Select Track'))
   for w,h in ([(140,42)] if phase in ('before','actions') else [(140,42),(100,28),(80,24),(40,12),(140,42)]):
    tm('resize-window','-t',target,'-x',str(w),'-y',str(h))
    query('cafe',1);save(f'{w}x{h}-{surface}-positive')
    query('a:Áurea',0 if phase=='before' else 4);save(f'{w}x{h}-{surface}-field')
    if phase!='before':
     for q,n in [('a:aurea -live',3),('al:horizontes',2),('t:hsm',2),('f:cancion',1),('t:夜に',1),('a:live',0)]:query(q,n)
     save(f'{w}x{h}-{surface}-none')
     query('',6);wait('a: · al: · t: · f: · -live');save(f'{w}x{h}-{surface}-examples')
    results.append({'language':lang,'profile':profile,'color':color,'surface':surface,'size':[w,h],'ok':True})
   if phase!='before':
    query('f:02',1)
    oldqueue=ipc('queue').get('queue',[]);key('Tab')
    until(lambda: [x['path'] for x in ipc('queue').get('queue',[])]==[x['path'] for x in oldqueue]+[paths[1]])
    wait('f:02');save(f'{surface}-tab')
    cli('play',paths[0]);until(lambda:ipc('status').get('status',{}).get('track',{}).get('path')==paths[0])
    key('Enter')
    until(lambda: ipc('status').get('status',{}).get('track',{}).get('path')==paths[1])
    results.append({'surface':surface,'action':'Tab/Enter','added_path':paths[1],'played_path':paths[1],'ok':True})
   else:key('Escape')
   if surface=='songs':wait('Biblioteca' if lang=='es' else 'Library')
   else:tm('kill-window','-t','ux:select');target='ux:main'
 finally:
  if target=='ux:select':
   key('C-c');tm('kill-window','-t','ux:select')
  target='ux:main';key('C-c')
  until(lambda:not (r/'run/maly/maly.sock').exists())
 (dest/'matrix.json').write_text(json.dumps(results,indent=2));print(lang,profile,'OK',flush=True)
