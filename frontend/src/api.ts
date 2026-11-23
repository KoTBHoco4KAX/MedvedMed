import type { AuthSession, ChatState, HistoryItem } from './types'

const TOKEN_KEY = 'medved.sessionToken'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function readToken(): string {
  try {
    return sessionStorage.getItem(TOKEN_KEY) ?? ''
  } catch {
    return ''
  }
}

export function saveToken(token: string): void {
  try {
    sessionStorage.setItem(TOKEN_KEY, token)
  } catch {
    // приватный режим браузера — работаем без sessionStorage
  }
}

export function dropToken(): void {
  try {
    sessionStorage.removeItem(TOKEN_KEY)
  } catch {
    // ignore
  }
}

function errorMessage(data: unknown, status: number): string {
  if (typeof data === 'object' && data !== null && 'error' in data) {
    const { error } = data as { error?: unknown }
    if (typeof error === 'string' && error.trim() !== '') return error
  }
  return `Ошибка сервера (${status})`
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'Сервер недоступен. Проверьте соединение и попробуйте ещё раз.')
  }

  const raw = await response.text()
  let data: unknown = null
  if (raw !== '') {
    try {
      data = JSON.parse(raw)
    } catch {
      data = null
    }
  }

  if (!response.ok) {
    throw new ApiError(response.status, errorMessage(data, response.status))
  }
  return data as T
}

function sessionHeaders(token: string): Record<string, string> {
  return { Accept: 'application/json', 'X-Session-Token': token }
}

// POST /api/auth — обмен токена MAX на серверную сессию.
export function auth(token: string): Promise<AuthSession> {
  return request<AuthSession>('/api/auth', {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
}

// GET /api/chat/state — текущее состояние опроса.
export function fetchChatState(token: string): Promise<ChatState> {
  return request<ChatState>('/api/chat/state', { headers: sessionHeaders(token) })
}

// POST /api/chat/answer — ответ пользователя и шаг вперёд.
export function sendAnswer(token: string, answer: string): Promise<ChatState> {
  return request<ChatState>('/api/chat/answer', {
    method: 'POST',
    headers: { ...sessionHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ answer }),
  })
}

// POST /api/chat/back — возврат на предыдущий шаг.
export function goBack(token: string): Promise<ChatState> {
  return request<ChatState>('/api/chat/back', { method: 'POST', headers: sessionHeaders(token) })
}

// POST /api/chat/reset — новый опрос с шага disclaimer.
export function resetChat(token: string): Promise<ChatState> {
  return request<ChatState>('/api/chat/reset', { method: 'POST', headers: sessionHeaders(token) })
}

// GET /api/history — прошлые консультации пользователя.
export function fetchHistory(token: string): Promise<HistoryItem[]> {
  return request<HistoryItem[]>('/api/history', { headers: sessionHeaders(token) })
}
