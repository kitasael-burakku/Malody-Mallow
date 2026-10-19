"""Neutralizaciones compilables, una por vez; restaura SIEMPRE con cp.
python3 reverse.py. No necesita mpv ni toca el índice de Git.
"""
import pathlib as P, tempfile, subprocess as S, os, json, sys
root=P.Path.cwd();dest=P.Path(__file__).resolve().parent;backup=P.Path(tempfile.mkdtemp(prefix='mu7reverse-'))
checks=[('nueva_descarga', 'internal/tui/console.go', 'm.getGen++ // una nueva descarga', '// una nueva descarga', './internal/tui', 'TestNewDownloadInvalidatesPlaylist', 'descarga nueva no invalidó'), ('parcial', 'internal/getter/outcome.go', 'reported := ReadPaths(pathsFile)', 'if runErr != nil { return Outcome{}, runErr }; reported := ReadPaths(pathsFile)', './internal/getter', 'TestDownloadOutcome', 'rutas equivocadas'), ('limite_rutas', 'internal/getter/outcome.go', 'err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))', 'err != nil || (false && (filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))))', './internal/getter', 'TestDownloadExactPaths', 'aceptó ruta ajena'), ('indexacion', 'internal/getter/outcome.go', 'if !ok {', 'if !ok && false {', './internal/getter', 'TestDownloadIndexedMissing', 'habilitó un archivo sin indexar'), ('ruta_relativa', 'internal/getter/outcome.go', 'dir = absDir', '_ = absDir', './internal/getter', 'TestDownloadRelativeDirectory', 'download left no track'), ('ruta_accion', 'internal/tui/get_finish.go', 'Paths: []string{it.value}', 'Paths: []string{it.label}', './internal/tui', 'TestDownloadContinuation', 'añadido incorrecto'), ('error_scan', 'internal/tui/get_finish.go', 'd.err = i18n.T("get.scan_failed") + "\\n" + msg.err.Error()', 'd.err = ""', './internal/tui', 'TestDownloadScanFailureAndRetry', 'ocultó fallo parcial/scan'), ('respuesta_obsoleta', 'internal/tui/get_finish.go', ' || msg.gen != m.getGen', '', './internal/tui', 'TestDownloadedPlaylistGeneration', 'scan viejo reemplazó otra descarga'), ('playlist_obsoleta', 'internal/tui/tui.go', ' && msg.downloadGen == m.getGen', '', './internal/tui', 'TestDownloadedPlaylistGeneration', 'playlist tardía reabrió el selector'), ('aviso_obsoleto', 'internal/tui/get_finish.go', 'm.flash = ""', 'm.flash = m.flash', './internal/tui', 'TestDownloadContinuation', 'conservó un aviso ajeno'), ('cli_existente', 'cmd/maly/get.go', 'fmt.Println(out.Summary())', 'fmt.Println(out.Summary()); if out.Existing > 0 { return nil }', './cmd/maly', 'TestGetContinuationOutcomes', 'library.db: no such file or directory')]
results=[];logs=[]
if len(sys.argv)>1:
 checks=[c for c in checks if c[0] in sys.argv[1:]]
 results=json.loads((dest/'reverse.json').read_text());logs=[(dest/'reverse.log').read_text()]
env=os.environ|{'CGO_ENABLED':'0'}
for name,file,old,new,pkg,test,expected in checks:
 p=root/file;b=backup/name;S.run(['cp',str(p),str(b)],check=True)
 try:
  source=p.read_text();assert old in source;p.write_text(source.replace(old,new,1))
  args=['go','test',pkg,'-run','^'+test+'$','-count=1'];r=S.run(args,env=env,capture_output=True,text=True);out=r.stdout+r.stderr
  logs.append(name+' — neutralizado\n'+out)
  assert r.returncode!=0 and '[build failed]' not in out and expected in out,(name,out)
 finally:S.run(['cp',str(b),str(p)],check=True)
 r=S.run(args,env=env,capture_output=True,text=True);logs.append(name+' — restaurado\n'+r.stdout+r.stderr);assert r.returncode==0,r.stdout+r.stderr
 results.append({'case':name,'neutralized':'failed for expected assertion, compiled','restored':'passed'})
 (dest/'reverse.log').write_text('\n'.join(logs));(dest/'reverse.json').write_text(json.dumps(results,indent=2,ensure_ascii=False));print(name,'OK',flush=True)
