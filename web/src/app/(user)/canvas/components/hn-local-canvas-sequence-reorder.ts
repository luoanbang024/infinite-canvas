import { isHNProjectId } from "@/services/hn/local-reference";
import { executeReorder, initializeReorder, lookupReorder, readReorderSnapshot, type ReorderCommand, type ReorderSnapshot, type ReorderReceipt } from "@/services/hn/local-sequence-reorder";
import { browserReorderJournal, readReorderJournal, sealReorderRecord, sameReorderCommand, unresolvedReorder, type ReorderJournal, type ReorderJournalRecord } from "./hn-local-sequence-reorder-journal";
export const HN_REORDER_LOAD = "加载安全排序";
export const HN_REORDER_EXPLANATION = "此操作可能初始化本地安全排序元数据，不改变剪辑顺序，不生成或导出媒体，不代表 AI/Provider 成功。";
export const HN_REORDER_CONFIRMATION = "仅改变本地主序列顺序，不生成或导出视频。命令绑定当前 sequenceRevision；若其他操作已改变序列，服务器将返回冲突，不覆盖较新的状态。响应不会自动重试。";
export const HN_REORDER_UNKNOWN = "排序结果未确认，可能已经提交；不会自动重试。";
export const HN_REORDER_CONTINUATION = "若此前未提交，此操作可能现在修改主序列顺序；若已经提交，只返回原回执，不会重复执行。若当前序列版本已经变化且此前未提交，此命令将返回冲突，不会覆盖较新的序列状态。这是明确的写入操作，不是只读确认；使用同一个 intent 和完全相同的命令。";
export type ReorderEntry = { running:boolean; records:ReorderJournalRecord[]; snapshot:ReorderSnapshot|null; draft:string[]; currentVerified:boolean; storageBlocked:boolean; error:string|null; laterChanges:boolean };
type Dependencies = { journal?:ReorderJournal; request?:typeof fetch; uuid?:()=>string; now?:()=>string };
export class HNLocalSequenceReorderController {
 private entries = new Map<string,ReorderEntry>();
 private readonly journal:ReorderJournal; private readonly request:typeof fetch; private readonly uuid:()=>string; private readonly now:()=>string;
 constructor(private readonly changed:()=>void = ()=>{}, deps:Dependencies = {}) { this.journal=deps.journal||browserReorderJournal;this.request=deps.request||fetch;this.uuid=deps.uuid||(()=>crypto.randomUUID());this.now=deps.now||(()=>new Date().toISOString()); }
 private state(project:string) { let e=this.entries.get(project); if(!e){ e={running:false,records:[],snapshot:null,draft:[],currentVerified:false,storageBlocked:false,error:null,laterChanges:false};this.entries.set(project,e); }return e; }
 entry(project:string):ReorderEntry { return structuredClone(this.state(project)); }
 canSubmit(project:string) { const e=this.state(project), ids=e.snapshot?.items.map(i=>i.sequenceItemId);return isHNProjectId(project)&&!e.running&&!e.storageBlocked&&!e.records.some(unresolvedReorder)&&!!ids&&ids.length>1&&ids.length<=256&&ids.length===e.draft.length&&new Set(e.draft).size===ids.length&&e.draft.every(id=>ids.includes(id))&&JSON.stringify(ids)!==JSON.stringify(e.draft); }
 move(project:string,id:string,delta:-1|1){ const e=this.state(project),at=e.draft.indexOf(id),to=at+delta;if(e.running||e.storageBlocked||e.records.some(unresolvedReorder)||at<0||to<0||to>=e.draft.length)return;[e.draft[at],e.draft[to]]=[e.draft[to],e.draft[at]];this.changed(); }
 private async run(project:string,action:(e:ReorderEntry)=>Promise<boolean>){ if(!isHNProjectId(project))return false;const e=this.state(project);if(e.running)return false;e.running=true;e.error=null;this.changed(); // synchronous page/project/main lock before first await
  try{return await action(e);}catch{e.error="安全排序操作未完成；不会自动发送或重试。";return false;}finally{e.running=false;this.changed();} }
 private put(e:ReorderEntry,r:ReorderJournalRecord){e.records=[...e.records.filter(x=>x.command.reorderIntentId!==r.command.reorderIntentId),structuredClone(r)].sort((a,b)=>a.createdAt.localeCompare(b.createdAt)||a.command.reorderIntentId.localeCompare(b.command.reorderIntentId));}
 private async scan(project:string,e:ReorderEntry){try{const records=await readReorderJournal(this.journal,project);
  for(const old of e.records.filter(unresolvedReorder)){const found=records.find(r=>r.command.reorderIntentId===old.command.reorderIntentId);if(!found)records.push(old);else if(!sameReorderCommand(old.command,found.command))throw Error("identity");else if(found.state==="PREPARED"||found.state==="ABORTED_PRE_DISPATCH")records[records.indexOf(found)]=old;}
  e.records=records;e.storageBlocked=false;return true;}catch{e.storageBlocked=true;e.error="排序日志无法可靠读取；已停止新排序。";return false;} }
 inspect(project:string){return this.run(project,e=>this.scan(project,e));} // Journal-only, zero HTTP/write.
 private async snapshot(project:string,e:ReorderEntry,receipt?:ReorderReceipt){e.currentVerified=false;try{const s=await readReorderSnapshot(project,this.request);e.snapshot=s;e.draft=s.items.map(i=>i.sequenceItemId);e.currentVerified=true;e.laterChanges=!!receipt&&receipt.outcome==="COMMITTED"&&(BigInt(s.sequenceRevision)>BigInt(receipt.appliedRevision!)||JSON.stringify(e.draft)!==JSON.stringify(receipt.desiredSequenceItemIds));return true;}catch{e.error="当前顺序未读取；历史命令回执仍然保留。";return false;} }
 load(project:string){return this.run(project,async e=>{if(!await this.scan(project,e))return false;await initializeReorder(project,this.request);return this.snapshot(project,e);});}
 refresh(project:string){return this.run(project,e=>this.snapshot(project,e));}
 private async terminal(project:string,e:ReorderEntry,r:ReorderJournalRecord,receipt:ReorderReceipt){const sealed=await sealReorderRecord(this.journal,{...r,state:receipt.outcome,receipt,updatedAt:this.now()});this.put(e,sealed);this.changed();await this.snapshot(project,e,receipt);return true;}
 private async dispatch(project:string,e:ReorderEntry,r:ReorderJournalRecord,continuation:boolean){let dispatched=false,latest=r;
  const request=(async(input:Parameters<typeof fetch>[0],init?:RequestInit)=>{if(init?.method==="POST"){const u=new URL(String(input));if(u.pathname!==`/api/hn/projects/${encodeURIComponent(project)}/sequences/main/reorder-commands`||init.body!==JSON.stringify(r.command)||dispatched)throw Error("dispatch identity");latest=await sealReorderRecord(this.journal,{...r,state:"DISPATCHING",receipt:null,updatedAt:this.now()});this.put(e,latest);e.currentVerified=false;dispatched=true;}return this.request(input,init);}) as typeof fetch;
  try{const receipt=await executeReorder(project,r.command,request);return await this.terminal(project,e,latest,receipt);}catch{const state=dispatched||continuation?"UNKNOWN":"ABORTED_PRE_DISPATCH";latest={...latest,state,receipt:null,updatedAt:this.now()};this.put(e,latest);try{this.put(e,await sealReorderRecord(this.journal,latest));}catch{/* Durable DISPATCHING or memory barrier remains unresolved. */}e.error=state==="UNKNOWN"?HN_REORDER_UNKNOWN:"发送前已停止；排序 POST 为零。请检查本机服务或日志存储。";return false;}
 }
 submit(project:string,confirm:(command:ReorderCommand)=>Promise<boolean>){if(!this.canSubmit(project))return Promise.resolve(false);return this.run(project,async e=>{if(!await this.scan(project,e)||e.records.some(unresolvedReorder))return false;
  const preview:ReorderCommand={protocolVersion:1,reorderIntentId:"00000000-0000-4000-8000-000000000000",expectedRevision:e.snapshot!.sequenceRevision,desiredSequenceItemIds:[...e.draft]};
  if(!await confirm(structuredClone(preview)))return false;if(!await this.scan(project,e)||e.records.some(unresolvedReorder))return false;
  const now=this.now(),r:ReorderJournalRecord={journalVersion:1,projectId:project,sequenceId:"main",createdAt:now,updatedAt:now,command:{...preview,reorderIntentId:this.uuid()},state:"PREPARED",receipt:null};
  try{this.put(e,await sealReorderRecord(this.journal,r));}catch{e.storageBlocked=true;e.error="排序日志未可靠保存；排序 POST 为零。";return false;}return this.dispatch(project,e,r,false);
 });}
 lookup(project:string,intent:string){return this.run(project,async e=>{if(!await this.scan(project,e))return false;const r=e.records.find(r=>r.command.reorderIntentId===intent&&unresolvedReorder(r));if(!r)return false;try{const receipt=await lookupReorder(project,r.command,this.request);if(receipt.outcome==="NOT_OBSERVED"){e.error=HN_REORDER_UNKNOWN;return false;}return await this.terminal(project,e,r,receipt);}catch{e.error=HN_REORDER_UNKNOWN;return false;}});}
 continueSame(project:string,intent:string,confirm:(command:ReorderCommand)=>Promise<boolean>){return this.run(project,async e=>{if(!await this.scan(project,e))return false;const r=e.records.find(r=>r.command.reorderIntentId===intent&&unresolvedReorder(r));if(!r||!await confirm(structuredClone(r.command)))return false;return this.dispatch(project,e,r,true);});}
}
