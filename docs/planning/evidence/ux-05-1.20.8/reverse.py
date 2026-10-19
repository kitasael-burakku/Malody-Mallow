"""Regresiones bidireccionales. Ejecutar bajo bus privado desde la raíz del repo.
Restaura siempre producción con cp; no modifica el índice ni usa checkout/reset.
"""
import os, pathlib as P, subprocess as S, tempfile
root=P.Path(tempfile.mkdtemp(prefix='mu5r-'));dest=P.Path(__file__).resolve().parent
env=os.environ.copy();env['CGO_ENABLED']='0'
for k,d in [('XDG_CONFIG_HOME','cfg'),('XDG_DATA_HOME','data'),('XDG_RUNTIME_DIR','run')]:
 (root/d).mkdir();env[k]=str(root/d)
variants=[
 ('console-base','internal/tui/console.go',None,['./internal/tui','-run','TestUX05','-count=1']),
 ('metadata-without-args','internal/commandmeta/commands.go',('return e.Name + " " + i18n.T(e.ArgsKey)','_ = i18n.T(e.ArgsKey)\n return e.Name'),['./internal/commandmeta','./cmd/maly','-run','TestMetadataContracts|TestUX05CommandMetadataAndCompletion','-count=1']),
 ('names-only','internal/tui/console_suggestions.go',('if !match {','if !match { continue }; if !match {'),['./internal/tui','-run','TestUX05FindAndFill','-count=1']),
]
logs=[]
for name,path,replace,args in variants:
 path=P.Path(path);backup=root/(name+'.go');S.run(['cp',str(path),str(backup)],check=True)
 try:
  if replace:
   source=path.read_text();assert replace[0] in source;path.write_text(source.replace(*replace,1))
  else:path.write_bytes(S.check_output(['git','show','c8c5cf0:internal/tui/console.go']))
  p=S.run(['go','test',*args],env=env,stdout=S.PIPE,stderr=S.STDOUT,text=True)
  logs.append(name+'\n'+p.stdout)
  assert p.returncode!=0 and '[build failed]' not in p.stdout and 'FAIL' in p.stdout,(name,p.stdout)
  print(name,': compila y falla por comportamiento',flush=True)
 finally:S.run(['cp',str(backup),str(path)],check=True)
(dest/'reverse.log').write_text('\n'.join(logs))
p=S.run(['go','test','./internal/tui','./internal/commandmeta','./cmd/maly','-run','TestUX05|TestMetadataContracts','-count=1'],env=env,stdout=S.PIPE,stderr=S.STDOUT,text=True)
(dest/'restored.log').write_text(p.stdout);print(p.stdout);assert p.returncode==0
