import type { Language, Submission } from "./types";

const BASE : string = import.meta.env.VITE_API_URL ?? "/api";
export class ApiError extends Error {
    status : number;
    constructor(status:number,message:string){
        super(message);
        this.status = status;
    }
}

async function request<T>(path:string , options:{method?:string;body?:unknown;token?:string}={}):Promise<T>{
    const headers: Record<string,string>={};
    if (options.body !== undefined) headers["Content-Type"]="application/json";
    if (options.token) headers["Authorization"]=`Bearer ${options.token}`;

    let res : Response;
    try {
        res = await fetch(BASE + path, {
        method: options.method ?? "GET",
        headers,
        body: options.body === undefined ? undefined : JSON.stringify(options.body),
        });
    } 
    catch {
    throw new ApiError(0, "Cannot reach the server. Check your internet and try again.");
    }

    // The Go API sends errors as {"error": "..."}.
    const data = await res.json().catch(() => null);
    if (!res.ok) {
        throw new ApiError(res.status, data?.error ?? `Request failed (${res.status})`);
    }
    return data as T;
}


export function register(email: string, password: string) {
  return request<{ id: number; email: string }>("/auth/register", {
    method: "POST",
    body: { email, password },
  });
}

export async function login(email: string, password: string): Promise<string> {
  const data = await request<{ token: string }>("/auth/login", {
    method: "POST",
    body: { email, password },
  });
  return data.token;
}

export function runCode(token: string, language: Language, code: string) {
  return request<Submission>("/run", {
    method: "POST",
    body: { language, code },
    token,
  });
}

export function listSubmissions(token: string) {
  return request<Submission[]>("/submissions", { token });
}