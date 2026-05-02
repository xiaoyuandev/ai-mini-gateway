import type { ApiResult } from "./types"

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "").trim().replace(/\/+$/, "")

function resolvePath(path: string) {
  if (!apiBaseUrl) {
    return path
  }
  return `${apiBaseUrl}${path}`
}

function toDisplayBody(data: unknown, rawText: string) {
  if (data === null || data === undefined) {
    return rawText || ""
  }
  if (typeof data === "string") {
    return data
  }
  try {
    return JSON.stringify(data, null, 2)
  } catch {
    return rawText || String(data)
  }
}

export async function gatewayRequest<T = unknown>(
  method: string,
  path: string,
  options?: {
    headers?: Record<string, string>
    body?: unknown
  }
): Promise<ApiResult<T>> {
  const headers = { ...(options?.headers || {}) }
  const init: RequestInit = {
    method,
    headers
  }

  if (options?.body !== undefined) {
    init.body = typeof options.body === "string" ? options.body : JSON.stringify(options.body)
    if (!headers["Content-Type"] && !headers["content-type"]) {
      headers["Content-Type"] = "application/json"
    }
  }

  const response = await fetch(resolvePath(path), init)
  const rawText = await response.text()
  const contentType = response.headers.get("content-type") || ""

  let data: T | string | null = rawText
  if (contentType.includes("application/json") && rawText) {
    try {
      data = JSON.parse(rawText) as T
    } catch {
      data = rawText
    }
  } else if (!rawText) {
    data = null
  }

  return {
    ok: response.ok,
    status: response.status,
    statusText: response.statusText,
    contentType,
    data,
    rawText,
    displayBody: toDisplayBody(data, rawText),
    method,
    path
  }
}
