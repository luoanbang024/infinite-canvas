import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { executeReorder, initializeReorder, lookupReorder, readReorderSnapshot, reorderCanonical, validReorderCommand, validReorderReceipt, validSequenceRevision, type ReorderCommand, type ReorderReceipt } from "./local-sequence-reorder";

const project = "test", intent = "12345678-1234-4123-8123-123456789abc";
const command = (): ReorderCommand => ({protocolVersion:1,reorderIntentId:intent,expectedRevision:"0",desiredSequenceItemIds:["B","A"]});
const ok = (data: unknown) => Response.json({code:0,data,msg:"ok"});
async function receipt(c: ReorderCommand, outcome: "COMMITTED" | "CONFLICT" = "COMMITTED", observed = c.expectedRevision): Promise<ReorderReceipt> {
 const hash = await crypto.subtle.digest("SHA-256",new TextEncoder().encode(reorderCanonical(c)));
 return {protocolVersion:1,projectId:project,sequenceId:"main",reorderIntentId:c.reorderIntentId,canonicalRequestHash:Array.from(new Uint8Array(hash),(x)=>x.toString(16).padStart(2,"0")).join(""),expectedRevision:c.expectedRevision,outcome,observedRevision:observed,appliedRevision:outcome==="COMMITTED"?String(BigInt(observed)+BigInt(1)):null,desiredSequenceItemIds:[...c.desiredSequenceItemIds],errorClass:outcome==="CONFLICT"?"SEQUENCE_REVISION_CONFLICT":null};
}
function transport(actual: (url: string,init?: RequestInit)=>Promise<Response>) {
 const calls: {url:string;init?:RequestInit}[] = [];
 const request = (async (url: string | URL | Request,init?: RequestInit)=>{
  calls.push({url:String(url),init});
  assert.equal(init?.credentials,"omit");assert.equal(init?.redirect,"error");assert.equal(init?.cache,"no-store");assert.ok(init?.signal);
  if (String(url)==="/api/hn/local-endpoint") return ok({url:"http://127.0.0.1:8080"});
  assert.ok(String(url).startsWith("http://127.0.0.1:8080/api/hn/projects/test/sequences/main/"));
  assert.equal((init?.headers as Record<string,string>)["X-HN-Local-Request"],"1");
  return actual(String(url),init);
 }) as typeof fetch;
 return {request,calls};
}
describe("R30 typed transport only",()=>{
 test("canonical decimal signed-int64 strings and strict command preflight",async()=>{
  for (const v of ["0","1","9007199254740993","9223372036854775807"]) assert.equal(validSequenceRevision(v),true);
  for (const v of [0,1,undefined,"","00","01","+1","-1"," 1","1 ","1e2","9223372036854775808"]) assert.equal(validSequenceRevision(v),false);
  for (const c of [{...command(),expectedRevision:0},{...command(),reorderIntentId:intent.toUpperCase()},{...command(),reorderIntentId:"12345678-1234-3123-8123-123456789abc"},{...command(),desiredSequenceItemIds:["A","A"]},{...command(),desiredSequenceItemIds:["../bad"]},{...command(),desiredSequenceItemIds:Array.from({length:257},(_,i)=>String(i))},{...command(),extra:0}]) {
   assert.equal(validReorderCommand(c),false);let count = 0;
   await assert.rejects(executeReorder(project,c as ReorderCommand,(async()=>{count++;throw Error("unexpected")}) as typeof fetch));
   assert.equal(count,0);
  }
 });
 test("initialize, command, historical receipt and provisional exact lookup",async()=>{
  const c = command(), r = await receipt(c);
  const fake = transport(async(url,init)=>{
   if(url.endsWith("initialize")){assert.deepEqual(JSON.parse(String(init?.body)),{protocolVersion:1});return ok({protocolVersion:1,projectId:project,sequenceId:"main",sequenceRevision:"9007199254740993"});}
   if(url.endsWith("/"+intent)){assert.equal(init?.method,"GET");return ok(r);}
   assert.equal(init?.method,"POST");assert.deepEqual(JSON.parse(String(init?.body)),c);return ok(r);
  });
  assert.equal((await initializeReorder(project,fake.request)).sequenceRevision,"9007199254740993");
  assert.deepEqual(await executeReorder(project,c,fake.request),r);
  assert.deepEqual(await lookupReorder(project,c,fake.request),r);
  assert.equal(fake.calls.filter(x=>x.init?.method==="POST").length,2);
  const absent = transport(async()=>ok({protocolVersion:1,projectId:project,sequenceId:"main",reorderIntentId:intent,outcome:"NOT_OBSERVED"}));
  assert.equal((await lookupReorder(project,c,absent.request)).outcome,"NOT_OBSERVED");
  assert.equal(absent.calls.some(x=>x.init?.method==="POST"),false);
 });
 test("terminal409 conflict validated, no fallback or resend",async()=>{
  const c=command(), r=await receipt(c,"CONFLICT","2");
  const fake=transport(async()=>Response.json({code:1,data:r,msg:r.errorClass},{status:409}));
  assert.deepEqual(await executeReorder(project,c,fake.request),r);
  assert.equal(fake.calls.filter(x=>x.init?.method==="POST").length,1);
  assert.equal(fake.calls.some(x=>x.url.endsWith("/reorder")),false);
  assert.equal(await validReorderReceipt({...r,observedRevision:"0"},project,c),false);
  assert.equal(await validReorderReceipt({...r,canonicalRequestHash:"0".repeat(64)},project,c),false);
  assert.equal(await validReorderReceipt({...r,extra:1},project,c),false);
 });
 test("lost/invalid/timeout response never triggers automaticPOST",async()=>{
  for (const kind of ["drop","malformed","identity","oversize","status"]) {
   const c=command();const fake=transport(async()=>{
    if(kind==="drop")throw Error("synthetic timeout/drop");
    if(kind==="malformed")return new Response("{");
    if(kind==="oversize")return new Response("x".repeat(1_048_577));
    if(kind==="status")return Response.json({code:1,data:null,msg:"synthetic_private"},{status:500});
    return ok({...await receipt(c),projectId:"other"});
   });
   await assert.rejects(executeReorder(project,c,fake.request));
   assert.equal(fake.calls.filter(x=>x.init?.method==="POST").length,1);
  }
 });
 test("capture exact payload before discovery await",async()=>{
  const c=command(), original=structuredClone(c), r=await receipt(original);let resolve!: (r:Response)=>void;const waiting=new Promise<Response>(r=>resolve=r);
  const calls: RequestInit[]=[];
  const request=(async(url: string | URL | Request,init?:RequestInit)=>{
   if(String(url)==="/api/hn/local-endpoint")return waiting;
   calls.push(init!);return ok(r);
  }) as typeof fetch;
  const done=executeReorder(project,c,request);c.expectedRevision="9";c.desiredSequenceItemIds[0]="changed";
  resolve(ok({url:"http://127.0.0.1:8080"}));
  assert.deepEqual(await done,r);assert.deepEqual(JSON.parse(String(calls[0].body)),original);assert.equal(calls.length,1);
 });
 test("bounded closed full snapshot, string revision, unique sorted IDs/ordinals",async()=>{
  const item={sequenceItemId:"A",projectId:project,sequenceId:"main",orderIndex:0,shotId:"shot",candidateId:"candidate",resultId:"result",createdAt:"2026-10-07T00:00:00Z",updatedAt:"2026-10-07T00:00:00Z"};
  const good={protocolVersion:1,projectId:project,sequenceId:"main",itemsComplete:true,sequenceRevision:"1",items:[item]};
  assert.deepEqual(await readReorderSnapshot(project,transport(async()=>ok(good)).request),good);
  for(const bad of [{...good,extra:0},{...good,sequenceRevision:1},{...good,itemsComplete:false},{...good,items:[item,item]},{...good,items:[item,{...item,sequenceItemId:"B"}]},{...good,items:[{...item,updatedAt:"2026-02-30T00:00:00Z"}]},{...good,items:Array.from({length:257},()=>item)}]) await assert.rejects(readReorderSnapshot(project,transport(async()=>ok(bad)).request));
 });
 test("discovery rejects nonlocal URL before actual command, no credential routes",async()=>{
  let calls=0;const req=(async()=>{calls++;return ok({url:"https://synthetic.invalid"});}) as typeof fetch;
  await assert.rejects(executeReorder(project,command(),req));assert.equal(calls,1);
 });
});

