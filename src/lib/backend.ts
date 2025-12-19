export const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL || "http://localhost:8080";
export const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "";
export const BALANCE_POLL_MS = 10000;
export const CREDIT_SCALE = 10000;

export function formatCredits(units: number | string | null | undefined) {
  const num = Number(units || 0);
  return (num / CREDIT_SCALE).toFixed(4);
}

export function authHeaders(token?: string, extra: Record<string, string> = {}) {
  if (!token) return { ...extra };
  return { ...extra, Authorization: `Bearer ${token}` };
}

export async function fetchJson<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const res = await fetch(`${BACKEND_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init.headers,
      ...authHeaders(token),
    },
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed with ${res.status}`);
  }

  return res.json() as Promise<T>;
}
