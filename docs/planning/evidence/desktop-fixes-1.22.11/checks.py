"""Cierre: dbus-run-session -- python3 checks.py. Aísla XDG y mpv (ao=null)."""
import os,subprocess as S,tempfile,pathlib as P,json,time,shutil
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS')
root=P.Path(tempfile.mkdtemp(prefix='mupc-'));env=os.environ.copy()
for key,d in [('XDG_CONFIG_HOME','cfg'),('XDG_DATA_HOME','data'),('XDG_RUNTIME_DIR','run'),('XDG_CACHE_HOME','cache')]:
 (root/d).mkdir();env[key]=str(root/d)
(root/'cfg/mpv').mkdir();(root/'cfg/mpv/mpv.conf').write_text('ao=null\n')
(root/'cfg/maly').mkdir();(root/'cfg/maly/config.toml').write_text('language="es"\nupdate_check=false\n[visualizer]\nenabled=false\n')
dest=P.Path(__file__).resolve().parent
result={'mpv':shutil.which('mpv'),'bus':'private','checks':{}}
for name,args,cgo in [
 ('build',['go','build','-o',str(root/'maly'),'./cmd/maly'],'0'),
 ('vet',['go','vet','./...'],'0'),
 ('suite',['go','test','-json','-count=1','./...'],'0'),
 ('race',['go','test','-race','-json','-count=1','./...'],'1'),
]:
 start=time.monotonic();p=S.run(args,env=env|{'CGO_ENABLED':cgo},stdout=S.PIPE,stderr=S.STDOUT,text=True)
 (dest/(name+'.log')).write_text(p.stdout)
 data={'exit':p.returncode,'seconds':round(time.monotonic()-start,2),'command':args,'CGO_ENABLED':cgo}
 if name in ('suite','race'):
  events=[]
  for line in p.stdout.splitlines():
   try:events.append(json.loads(line))
   except json.JSONDecodeError:pass
  data.update(packages=sum(x.get('Action')=='pass' and 'Test' not in x for x in events),cases=sum(x.get('Action')=='pass' and 'Test' in x for x in events),skips=[x for x in events if x.get('Action')=='skip'],failures=[x for x in events if x.get('Action')=='fail'])
 result['checks'][name]=data;(dest/'checks.json').write_text(json.dumps(result,indent=2,ensure_ascii=False));print(name,data,flush=True)
 assert p.returncode==0,p.stdout[-3000:]
