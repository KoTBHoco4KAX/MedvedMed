import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import {
  ApiError,
  auth,
  dropToken,
  fetchChatState,
  fetchHistory,
  goBack,
  readToken,
  resetChat,
  saveToken,
  sendAnswer,
} from './api'
import type { ChatMessage, ChatState, HistoryItem } from './types'
import { Header, ProgressBar } from './components/layout/Header'
import { MessageBubble, TypingBubble } from './components/chat/MessageBubble'
import { Composer } from './components/chat/Composer'
import { ResultCard } from './components/result/ResultCard'
import { EmergencyCard } from './components/result/EmergencyCard'
import { HistoryPanel } from './components/result/HistoryPanel'
import { EmergencyGuideModal } from './components/modals/EmergencyGuideModal'
import { BodyMapModal } from './components/modals/BodyMapModal'

type Phase = 'booting' | 'chat' | 'denied' | 'failed'
type View = 'chat' | 'history'
type Pending = null | 'answer' | 'back' | 'reset'

const EMERGENCY_FALLBACK_INSTRUCTION =
  'Вызовите скорую помощь (103 или 112) прямо сейчас и не ждите окончания опроса.'

function asApiError(error: unknown): ApiError {
  return error instanceof ApiError
    ? error
    : new ApiError(0, 'Не удалось выполнить запрос. Попробуйте ещё раз.')
}

export default function App() {
  const [phase, setPhase] = useState<Phase>('booting')
  const [notice, setNotice] = useState('')
  const [chat, setChat] = useState<ChatState | null>(null)
  const [view, setView] = useState<View>('chat')
  const [pending, setPending] = useState<Pending>(null)
  const [error, setError] = useState('')
  const [history, setHistory] = useState<HistoryItem[] | null>(null)
  const [historyError, setHistoryError] = useState('')
  const [guideOpen, setGuideOpen] = useState(false)
  const [bodyMapOpen, setBodyMapOpen] = useState(false)

  const feedRef = useRef<HTMLDivElement | null>(null)
  const booted = useRef(false)
  const bodyMapDismissed = useRef<string | null>(null)
  const lastStepRef = useRef<string | null>(null)

  const deny = useCallback((message: string) => {
    dropToken()
    setChat(null)
    setNotice(message)
    setPhase('denied')
  }, [])

  const loadChat = useCallback(async (token: string) => {
    setChat(await fetchChatState(token))
    setPhase('chat')
  }, [])

  const bootstrap = useCallback(async () => {
    const urlToken = (new URLSearchParams(window.location.search).get('token') ?? '').trim()
    if (urlToken !== '') {
      // Токен одноразовый: сразу убираем его из адресной строки.
      window.history.replaceState({}, '', window.location.pathname)
    }

    try {
      if (urlToken === '') {
        const stored = readToken()
        if (stored !== '') {
          try {
            await loadChat(stored)
            return
          } catch (error) {
            // Сессия в SQLite могла истечь — пробуем авторизоваться заново.
            if (!(error instanceof ApiError) || error.status !== 401) throw error
            dropToken()
          }
        }
      }

      const session = await auth(urlToken)
      saveToken(session.sessionToken)
      await loadChat(session.sessionToken)
    } catch (error) {
      const apiError = asApiError(error)
      if (apiError.status === 401 || apiError.status === 403 || apiError.status === 404) {
        deny(apiError.message)
        return
      }
      setNotice(apiError.message)
      setPhase('failed')
    }
  }, [deny, loadChat])

  useEffect(() => {
    // В StrictMode эффект вызывается дважды — авторизуемся ровно один раз.
    if (booted.current) return
    booted.current = true
    void bootstrap()
  }, [bootstrap])

  useEffect(() => {
    if (view !== 'chat') return
    const element = feedRef.current
    if (element === null) return

    // Итоговую карточку и красный флаг показываем с начала, а не с конца.
    const target =
      chat?.isEmergency === true
        ? element.querySelector<HTMLElement>('[data-card="emergency"]')
        : chat?.step === 'result'
          ? element.querySelector<HTMLElement>('[data-card="result"]')
          : null

    if (target !== null) {
      element.scrollTo({ top: Math.max(0, target.offsetTop - 12), behavior: 'smooth' })
      return
    }
    element.scrollTo({ top: element.scrollHeight, behavior: 'smooth' })
  }, [chat?.messages.length, chat?.step, chat?.isEmergency, pending, view])

  const tokenOrDeny = useCallback((): string | null => {
    const token = readToken()
    if (token === '') {
      deny('Сессия не найдена. Откройте сервис заново через бота в MAX.')
      return null
    }
    return token
  }, [deny])

  const fail = useCallback(
    (cause: unknown) => {
      const apiError = asApiError(cause)
      if (apiError.status === 401) {
        deny('Сессия истекла. Откройте сервис заново через бота в MAX.')
        return
      }
      setError(apiError.message)
    },
    [deny],
  )

  const submitAnswer = useCallback(
    async (raw: string) => {
      const answer = raw.trim()
      const current = chat
      if (answer === '' || pending !== null || current === null) return

      const token = tokenOrDeny()
      if (token === null) return

      const optimistic: ChatMessage = {
        role: 'user',
        step: current.step,
        text: answer,
        at: new Date().toISOString(),
      }
      setPending('answer')
      setError('')
      setChat({ ...current, messages: [...current.messages, optimistic] })

      try {
        setChat(await sendAnswer(token, answer))
      } catch (cause) {
        setChat(current)
        fail(cause)
      } finally {
        setPending(null)
      }
    },
    [chat, fail, pending, tokenOrDeny],
  )

  const handleBack = useCallback(async () => {
    if (pending !== null) return
    const token = tokenOrDeny()
    if (token === null) return

    setPending('back')
    setError('')
    bodyMapDismissed.current = null
    setBodyMapOpen(false)
    try {
      setChat(await goBack(token))
    } catch (cause) {
      fail(cause)
    } finally {
      setPending(null)
    }
  }, [fail, pending, tokenOrDeny])

  const handleReset = useCallback(async () => {
    if (pending !== null) return
    const token = tokenOrDeny()
    if (token === null) return

    setPending('reset')
    setError('')
    try {
      setChat(await resetChat(token))
      setView('chat')
      setHistory(null)
      setHistoryError('')
      bodyMapDismissed.current = null
      setBodyMapOpen(false)
    } catch (cause) {
      fail(cause)
    } finally {
      setPending(null)
    }
  }, [fail, pending, tokenOrDeny])

  const loadHistory = useCallback(async () => {
    const token = tokenOrDeny()
    if (token === null) return

    setHistory(null)
    setHistoryError('')
    try {
      setHistory(await fetchHistory(token))
    } catch (cause) {
      const apiError = asApiError(cause)
      if (apiError.status === 401) {
        deny('Сессия истекла. Откройте сервис заново через бота в MAX.')
        return
      }
      setHistory([])
      setHistoryError(apiError.message)
    }
  }, [deny, tokenOrDeny])

  const toggleHistory = useCallback(() => {
    setError('')
    if (view === 'history') {
      setView('chat')
      return
    }
    setView('history')
    void loadHistory()
  }, [loadHistory, view])

  const bodyMapKey =
    chat !== null && view === 'chat' && chat.step === 'complaint_location' && chat.question === 'Где именно болит?'
      ? `${chat.sessionToken}:${chat.step}:${chat.progress.current}`
      : null

  const closeBodyMap = useCallback(() => {
    if (bodyMapKey !== null) bodyMapDismissed.current = bodyMapKey
    setBodyMapOpen(false)
  }, [bodyMapKey])

  useEffect(() => {
    const step = chat?.step ?? null
    if (step === null) {
      lastStepRef.current = null
      setBodyMapOpen(false)
      return
    }
    if (step === lastStepRef.current) return

    lastStepRef.current = step
    bodyMapDismissed.current = null
    if (step === 'complaint_location' && chat?.question === 'Где именно болит?' && bodyMapDismissed.current !== bodyMapKey) {
      setBodyMapOpen(true)
    } else {
      setBodyMapOpen(false)
    }
  }, [bodyMapKey, chat?.question, chat?.step])

  const selectBodyMap = useCallback(
    (location: string) => {
      if (bodyMapKey !== null) bodyMapDismissed.current = bodyMapKey
      setBodyMapOpen(false)
      void submitAnswer(location)
    },
    [bodyMapKey, submitAnswer],
  )

  if (phase === 'booting') {
    return (
      <AuthShell>
        <div className="flex flex-col items-center gap-3 py-4 text-center">
          <span className="flex items-center gap-1.5" aria-hidden>
            <span className="size-1.5 animate-bounce rounded-full bg-brand [animation-delay:-0.3s]" />
            <span className="size-1.5 animate-bounce rounded-full bg-brand [animation-delay:-0.15s]" />
            <span className="size-1.5 animate-bounce rounded-full bg-brand" />
          </span>
          <p className="text-[15px] font-medium text-ink">Загружаем данные…</p>
          <p className="text-[13px] text-muted">Проверяем доступ и состояние опроса</p>
        </div>
      </AuthShell>
    )
  }

  if (phase === 'denied') {
    return (
      <AuthShell>
        <div className="text-center">
          <span aria-hidden className="block text-2xl">
            🔒
          </span>
          <h1 className="mt-4 text-lg leading-snug font-semibold text-ink">Откройте сервис через бота в MAX</h1>
          <p className="mt-2 text-[13px] leading-relaxed text-muted">{notice}</p>
          <p className="mt-2 text-[12px] leading-relaxed text-muted">
            Ссылка из бота содержит токен доступа. Без него вход невозможен — это защита данных пациента.
          </p>
          <button
            type="button"
            onClick={() => {
              setPhase('booting')
              void bootstrap()
            }}
            className="mt-5 h-11 w-full rounded-xl bg-brand text-[15px] font-medium text-white transition-colors hover:bg-brand-hover"
          >
            Попробовать снова
          </button>
        </div>
      </AuthShell>
    )
  }

  if (phase === 'failed') {
    return (
      <AuthShell>
        <div className="text-center">
          <span aria-hidden className="block text-2xl">
            ⚠️
          </span>
          <h1 className="mt-4 text-lg leading-snug font-semibold text-ink">Не удалось подключиться</h1>
          <p className="mt-2 text-[13px] leading-relaxed text-muted">{notice}</p>
          <button
            type="button"
            onClick={() => {
              setPhase('booting')
              void bootstrap()
            }}
            className="mt-5 h-11 w-full rounded-xl bg-brand text-[15px] font-medium text-white transition-colors hover:bg-brand-hover"
          >
            Повторить
          </button>
        </div>
      </AuthShell>
    )
  }

  const messages = chat === null ? [] : chat.messages.filter((item) => item.role === 'bot' || item.role === 'user')
  const showProgress = chat !== null && view === 'chat' && !chat.isEmergency
  const showComposer =
    chat !== null && view === 'chat' && !chat.isEmergency && !chat.completed && chat.step !== 'result'
  const lastStepIsFinal =
    chat !== null && chat.step !== 'result' && chat.progress.current >= chat.progress.total - 1

  return (
    <div className="mx-auto flex h-[100dvh] w-full max-w-md flex-col bg-canvas">
      <Header
        firstName={chat?.firstName ?? ''}
        historyOpen={view === 'history'}
        onToggleHistory={toggleHistory}
        onOpenGuide={() => setGuideOpen(true)}
      />
      {showProgress && chat !== null && (
        <ProgressBar
          current={chat.progress.current}
          total={chat.progress.total}
          canGoBack={chat.canGoBack}
          backDisabled={pending !== null}
          onBack={() => void handleBack()}
        />
      )}

      {view === 'history' ? (
        <HistoryPanel items={history} error={historyError} onRetry={() => void loadHistory()} />
      ) : (
        <>
          <div
            ref={feedRef}
            role="log"
            aria-live="polite"
            className="relative min-h-0 flex-1 overflow-y-auto px-4 py-4"
          >
            <div className="flex flex-col gap-3">
              {messages.map((message, index) => (
                <MessageBubble key={`${message.step}-${message.role}-${index}`} message={message} />
              ))}

              {pending === 'answer' && (
                <TypingBubble label={lastStepIsFinal ? 'Готовим карточку рекомендаций…' : 'Печатает…'} />
              )}

              {chat !== null && chat.isEmergency && (
                <EmergencyCard
                  reason={
                    chat.emergencyReason ??
                    'Вы описали симптомы, которые могут указывать на неотложное состояние.'
                  }
                  instruction={chat.emergencyInstruction ?? EMERGENCY_FALLBACK_INSTRUCTION}
                  onRestart={() => void handleReset()}
                />
              )}

              {chat !== null && !chat.isEmergency && chat.step === 'result' && chat.card !== undefined && (
                <ResultCard card={chat.card} onRestart={() => void handleReset()} />
              )}
            </div>
          </div>

          {error !== '' && (
            <div className="shrink-0 px-4 pb-2">
              <p
                role="alert"
                className="rounded-xl bg-white px-3 py-2 text-[12px] leading-relaxed text-red-700"
              >
                {error}
              </p>
            </div>
          )}

          {showComposer && chat !== null && (
            <Composer
              options={chat.options}
              allowFreeText={chat.allowFreeText}
              pending={pending !== null}
              showBodyMapButton={bodyMapKey !== null && !bodyMapOpen}
              onOpenBodyMap={() => setBodyMapOpen(true)}
              onAnswer={(answer) => void submitAnswer(answer)}
            />
          )}
        </>
      )}

      <EmergencyGuideModal open={guideOpen} onClose={() => setGuideOpen(false)} />
      <BodyMapModal
        open={bodyMapOpen && bodyMapKey !== null}
        sex={chat?.sex ?? 0}
        onClose={closeBodyMap}
        onSelect={selectBodyMap}
      />
    </div>
  )
}

function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="mx-auto flex h-[100dvh] w-full max-w-md flex-col bg-canvas">
      <div className="flex shrink-0 items-center bg-white px-4 py-3">
        <p className="text-[15px] font-semibold text-ink">Цифровой помощник</p>
      </div>
      <div className="flex flex-1 items-center px-4">
        <div className="w-full rounded-2xl bg-white p-6">{children}</div>
      </div>
    </div>
  )
}
