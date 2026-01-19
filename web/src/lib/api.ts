export type ApiError = {
  status: number;
  message: string;
  details?: unknown;
};

export type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
};

const DEFAULT_BASE = "/api";

function getBaseUrl() {
  const base = import.meta.env.VITE_API_BASE || DEFAULT_BASE;
  return base.endsWith("/") ? base.slice(0, -1) : base;
}

export function apiBase() {
  return getBaseUrl();
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const url = `${getBaseUrl()}${path}`;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...options.headers
  };

  const response = await fetch(url, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
    signal: options.signal
  });

  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json")
    ? await response.json()
    : await response.text();

  if (!response.ok) {
    const message =
      typeof payload === "string"
        ? payload
        : (payload as { message?: string; error?: string }).message ||
          (payload as { error?: string }).error ||
          "Request failed";
    const error: ApiError = {
      status: response.status,
      message,
      details: payload
    };
    throw error;
  }

  return payload as T;
}
