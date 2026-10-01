import { localBackendURL } from "@/services/hn/local-reference";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Read-only discovery of existing API_BASE_URL; no root, secrets or write proxy.
export async function GET() {
    try {
        const url = localBackendURL(process.env.API_BASE_URL || "http://127.0.0.1:8080");
        return Response.json({ code: 0, data: { url }, msg: "ok" }, { headers: { "Cache-Control": "no-store" } });
    } catch {
        return Response.json({ code: 1, data: null, msg: "HN 参考冻结需要本机后端地址" }, { status: 503 });
    }
}
