# Recorrido medido; adaptado a un binario por argumento y un temporal nuevo.
import os,subprocess as S,pathlib as P,time,json,sys,tempfile,shlex
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS'), 'Ejecutar bajo dbus-run-session'
root=P.Path(tempfile.mkdtemp(prefix='maly-u3-'));binary=str(P.Path(sys.argv[1]).resolve());phase='after';results=[]
print(root,flush=True)
for lang in (['es'] if phase=='before' else ['es','en']):
 for theme in (['no-color'] if phase=='before' else ['dark','light','no-color']):
  name=f'{phase}-{lang}-{theme}-r4';r=root/name
  for d in ['config/maly','config/mpv','data','run','music','caps']:(r/d).mkdir(parents=True,exist_ok=True)
  os.chmod(r/'run',0o700)
  cfg=f'language="{lang}"\nupdate_check=false\nscan_durations=false\nmusic_dir="{r}/music"\n[visualizer]\nenabled=false\n[theme]\nbanner="off"\n'
  if theme=='light':cfg+='text="#253035"\ndim="#506369"\naccent="#286565"\nplaying="#9b4337"\nerror="#9b4337"\nsurface="#dce8e8"\nborder="#acbabc"\naccent_dim="#5d7a7a"\n'
  (r/'config/maly/config.toml').write_text(cfg);(r/'config/mpv/mpv.conf').write_text('ao=null\n')
  e=os.environ.copy();e.update(XDG_CONFIG_HOME=str(r/'config'),XDG_DATA_HOME=str(r/'data'),XDG_RUNTIME_DIR=str(r/'run'),XDG_CACHE_HOME=str(r/'cache'),TERM='xterm-256color');e.pop('NO_COLOR',None)
  if theme=='no-color':e['NO_COLOR']='1'
  def cli(*args):return S.check_output([binary,*args],env=e,text=True,stderr=S.STDOUT)
  def tm(*args):return S.check_output(['tmux','-S',str(r/'tmux'),*args],env=e,text=True,stderr=S.STDOUT)
  def key(*a):tm('send-keys','-t','ux',*a)
  def cap():return tm('capture-pane','-p','-t','ux')
  def wait(*want, absent=()):
   deadline=time.monotonic()+12
   while time.monotonic()<deadline:
    out=cap()
    if all(x in out for x in want) and all(x not in out for x in absent):return out
    time.sleep(.05)
   raise AssertionError((name,want,out))
  def save(n):
   (r/'caps'/f'{n}.txt').write_text(cap());(r/'caps'/f'{n}.ansi').write_text(tm('capture-pane','-p','-e','-t','ux'))
  # Archivos de control: título largo, NFD, CJK, duplicado y metadatos ausentes.
  titles=['Una canción con un título muy largo que necesita respirar — versión acústica','Cafe\u0301 en 東京','Repetida','Repetida','Sin artista']
  ppm=r/'cover.ppm';ppm.write_bytes(b'P6\n32 32\n255\n'+bytes([40,160,160])*1024)
  for i,title in enumerate(titles):
   args=['ffmpeg','-v','error','-f','lavfi','-i','anullsrc=r=8000:cl=mono']
   if i==0:args+=['-i',str(ppm)]
   args+=['-t','240','-map','0:a','-c:a','flac']
   if i==0:args+=['-map','1:v','-c:v','png','-disposition:v','attached_pic']
   args+=['-metadata',f'title={title}']
   if i<4:args+=['-metadata','artist=Áurea','-metadata','album=Horizontes','-metadata',f'track={i+1}','-metadata','genre=Folk']
   S.run(args+['-y',str(r/f'music/{i}.flac')],check=True)
  (r/'music/0.lrc').write_text('[00:00.00]VERSO VISIBLE\n[00:20.00]Otro verso\n')
  tm('new-session','-d','-s','ux','-x','140','-y','42',shlex.quote(binary))
  try:
   wait('maly scan');cli('scan');wait('Áurea');cli('add',str(r/'music/0.flac'),str(r/'music/1.flac'));wait('Una canción');cli('pause');wait('⏸');wait('? ayuda' if lang=='es' else '? help')
   for w,h in [(140,42),(100,28),(80,24),(40,12),(140,42)]:
    tm('resize-window','-t','ux','-x',str(w),'-y',str(h));wait('? ayuda' if lang=='es' else '? help');key('/');key('-l','Una');key('Enter');wait('/Una', absent=('enter fija','enter accepts'));save(f'{w}x{h}-library')
    if phase=='after':assert '>▶' in cap(),cap()
    key('Escape');key('Tab');wait('Cola' if lang=='es' else 'Queue');key('Home');wait(('Cola' if lang=='es' else 'Queue')+' (2) · 1/2');
    if phase=='after':wait('>▶')
    save(f'{w}x{h}-queue-same')
    key('Down');
    if phase=='after':wait('> ', '▶')
    save(f'{w}x{h}-queue-other');key('Tab');key('C-o');wait('Canciones' if lang=='es' else 'Songs');save(f'{w}x{h}-songs');key('Escape');wait('Biblioteca (' if lang=='es' else 'Library (')
    key('C-t');wait('Ahora suena' if lang=='es' else 'Now Playing', ('esc cierra' if lang=='es' else 'esc closes') if phase!='before' else '', absent=('Biblioteca (','Library (','Canciones','Songs'));save(f'{w}x{h}-now');key('Escape');wait('Biblioteca (' if lang=='es' else 'Library (')
    results.append({'run':name,'size':f'{w}x{h}','ok':True})
  finally:
   key('C-c');deadline=time.monotonic()+10
   while (r/'run/maly/maly.sock').exists() and time.monotonic()<deadline:time.sleep(.05)
   assert not (r/'run/maly/maly.sock').exists()
  (root/f'{phase}-matrix.json').write_text(json.dumps(results,indent=2));print(name,'OK',flush=True)
