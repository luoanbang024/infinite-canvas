package foundation

import (
 "bytes"
 "os"
 "reflect"
 "testing"
)

func TestExportPlacementIndependentOfCurrentSelection(t *testing.T) {
 w := newWorkspace(t)
 s := makeShot(t,w,"same shot")
 var candidates []Candidate
 for _, label := range []string{"A","B"} {
  g:=makeGeneration(t,w,s.ID,label); r:=makeResult(t,w,g); archive(t,w,r,[]byte("synthetic-"+label))
  c,e:=w.CreateCandidate(s.ID,r.ID,label);okay(t,e);candidates=append(candidates,c)
 }
 okay(t,w.SelectCandidate(s.ID,candidates[0].ID))
 a,e:=w.AddSequenceItem("main",candidates[0].ID);okay(t,e)
 okay(t,w.SelectCandidate(s.ID,candidates[1].ID))
 b,e:=w.AddSequenceItem("main",candidates[1].ID);okay(t,e)
 repeat,e:=w.AddSequenceItem("main",candidates[1].ID);okay(t,e)
 order:=[]string{repeat.ID,a.ID,b.ID};okay(t,w.Reorder("main",order))
 snapshot:=func()map[string][]byte {
  result:=map[string][]byte{}
  for _,kind:=range []string{"shots","generations","results","archive_jobs","task_bindings","candidates","sequence_items"} {
   rows,e:=w.List(kind);okay(t,e);for n,row:=range rows { result[kind+string(rune(n))]=row }
  };return result
 }
 before:=snapshot(); first,e:=w.Export("main");okay(t,e)
 if !reflect.DeepEqual(before,snapshot()){t.Fatal("export mutated records")}
 path,e:=w.Resolve("exports/"+first.ExportID+"/ordered-manifest.json");okay(t,e);old,e:=os.ReadFile(path);okay(t,e)
 for n,i:=range first.Items {if i.SequenceItemID!=order[n] || i.SequenceIndex!=n+1 {t.Fatal("order")};okay(t,w.verifyFile(i.RelativePath,i.SHA256,i.ByteLength))}
 if first.Items[1].CandidateID!=candidates[0].ID || first.Items[0].ResultID!=first.Items[2].ResultID {t.Fatal("fallback/repeated placement")}
 okay(t,w.SelectCandidate(s.ID,"")); before=snapshot();second,e:=w.Export("main");okay(t,e)
 if second.ExportID==first.ExportID || !reflect.DeepEqual(before,snapshot()){t.Fatal("identity/record mutation")}
 for n,i:=range second.Items {if i.CandidateID!=first.Items[n].CandidateID {t.Fatal("selection fallback")}}
 okay(t,w.Reorder("main",[]string{a.ID,b.ID,repeat.ID}));third,e:=w.Export("main");okay(t,e)
 if third.Items[0].SequenceItemID!=a.ID || third.ExportID==first.ExportID {t.Fatal("new order/identity")}
 now,e:=os.ReadFile(path);okay(t,e);if !bytes.Equal(old,now){t.Fatal("old manifest changed")}
 okay(t,w.Close()); reopened,e:=Open(w.Root[:len(w.Root)-len(w.ProjectID)-1],w.ProjectID);okay(t,e);defer reopened.Close()
 for _,i:=range first.Items{okay(t,reopened.verifyFile(i.RelativePath,i.SHA256,i.ByteLength))}
 // Missing archive bytes remain a rejection, even with valid placements.
 r:=get[Result](t,reopened,"results",candidates[0].ResultID);media,e:=reopened.Resolve(r.ArchivedRelativePath);okay(t,e);okay(t,os.Remove(media));_,e=reopened.Export("main");reject(t,e)
}
