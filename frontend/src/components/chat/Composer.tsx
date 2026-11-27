import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import type { PointerEvent as ReactPointerEvent } from 'react'
import { optionEmoji } from '../../lib/format'

type ComposerProps = {
  options: string[]
  allowFreeText: boolean
  pending: boolean
  showBodyMapButton?: boolean
  onOpenBodyMap?: () => void
  onAnswer: (answer: string) => void
}

function speechRecognitionConstructor(): SpeechRecognitionConstructorLike | undefined {
  if (typeof window === 'undefined') return undefined
  return window.SpeechRecognition ?? window.webkitSpeechRecognition
}

function speechErrorMessage(error: string): string {
  switch (error) {
    case 'not-allowed':
    case 'service-not-allowed':
      return 'В мини-аппе MAX используйте микрофон 🎙️ на клавиатуре телефона'
    case 'no-speech':
      return 'Речь не распознана'
    case 'network':
      return 'Нет сети для распознавания'
    case 'audio-capture':
      return 'Микрофон недоступен'
    default:
      return 'Не удалось распознать речь'
  }
}

export function Composer({
  options,
  allowFreeText,
  pending,
  showBodyMapButton = false,
  onOpenBodyMap,
  onAnswer,
}: ComposerProps) {
  const [text, setText] = useState('')
  const [listening, setListening] = useState(false)
  const [speechHint, setSpeechHint] = useState('')
  const [speechSupported] = useState(() => speechRecognitionConstructor() !== undefined)
  const [optionsCollapsed, setOptionsCollapsed] = useState(false)
  const [sheetDragOffset, setSheetDragOffset] = useState(0)

  const recognitionRef = useRef<SpeechRecognitionLike | null>(null)
  const listeningRef = useRef(false)
  const baseTextRef = useRef('')
  const inputRef = useRef<HTMLInputElement | null>(null)
  const sheetDragStartRef = useRef<number | null>(null)
  const sheetDragMovedRef = useRef(false)
  const optionsKey = options.join('|')
  const hasQuickOptions = options.length > 0 || (showBodyMapButton && onOpenBodyMap !== undefined)

  // Компактная сетка для коротких/средних подписей; очень длинные варианты остаются в одну колонку.
  const grid = options.length > 4 && options.every((option) => option.length <= 32)

  const detachRecognition = useCallback((abort: boolean) => {
    const recognition = recognitionRef.current
    recognitionRef.current = null
    if (recognition === null) return
    try {
      if (abort) {
        recognition.abort()
      } else {
        recognition.stop()
      }
    } catch {
      try {
        recognition.abort()
      } catch {
        // Микрофон уже завершил работу — ничего не делаем.
      }
    }
  }, [])

  const stopSpeech = useCallback(() => {
    detachRecognition(false)
    listeningRef.current = false
    setListening(false)
  }, [detachRecognition])

  const startSpeech = useCallback(() => {
    if (pending || listeningRef.current) return
    const Recognition = speechRecognitionConstructor()
    if (Recognition === undefined) return

    detachRecognition(true)
    let recognition: SpeechRecognitionLike
    try {
      recognition = new Recognition()
    } catch {
      setSpeechHint('Не удалось запустить микрофон')
      return
    }
    recognition.lang = 'ru-RU'
    recognition.interimResults = true
    recognition.continuous = false
    recognition.maxAlternatives = 1

    recognition.onresult = (event) => {
      if (recognitionRef.current !== recognition) return
      let transcript = ''
      for (let index = 0; index < event.results.length; index += 1) {
        const alternative = event.results[index]?.[0]
        if (alternative !== undefined) transcript += alternative.transcript
      }
      const base = baseTextRef.current
      const recognized = transcript.trim()
      if (recognized === '') {
        setText(base)
      } else {
        setText(base === '' ? recognized : `${base} ${recognized}`)
      }
      setSpeechHint('')
    }

    recognition.onerror = (event) => {
      if (recognitionRef.current !== recognition) return
      recognitionRef.current = null
      listeningRef.current = false
      setListening(false)
      setSpeechHint(speechErrorMessage(event.error))
      if (event.error === 'not-allowed' || event.error === 'service-not-allowed') {
        inputRef.current?.focus()
      }
      try {
        recognition.abort()
      } catch {
        // Ошибка уже обработана — не пробрасываем её в UI.
      }
    }

    recognition.onend = () => {
      if (recognitionRef.current !== recognition) return
      recognitionRef.current = null
      listeningRef.current = false
      setListening(false)
    }

    baseTextRef.current = text.trim()
    try {
      recognition.start()
      recognitionRef.current = recognition
      listeningRef.current = true
      setListening(true)
      setSpeechHint('')
    } catch {
      recognitionRef.current = null
      listeningRef.current = false
      setListening(false)
      setSpeechHint('Не удалось запустить микрофон')
      try {
        recognition.abort()
      } catch {
        // start() уже завершился ошибкой — просто сбрасываем состояние.
      }
    }
  }, [detachRecognition, pending, text])

  const toggleSpeech = useCallback(() => {
    if (listeningRef.current) {
      stopSpeech()
      return
    }
    startSpeech()
  }, [startSpeech, stopSpeech])

  const handleTextChange = (value: string) => {
    setText(value)
    setSpeechHint('')
    if (listeningRef.current) stopSpeech()
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const value = text.trim()
    if (value === '' || pending) return
    stopSpeech()
    setSpeechHint('')
    onAnswer(value)
    setText('')
  }

  // Смена шага или режима ввода должна немедленно завершить диктовку.
  useEffect(() => {
    stopSpeech()
    setSpeechHint('')
  }, [allowFreeText, optionsKey, stopSpeech])

  useEffect(() => {
    if (pending) stopSpeech()
  }, [pending, stopSpeech])

  // На размонтировании не трогаем setState, только освобождаем микрофон.
  // На новом шаге сразу показываем варианты ответа, даже если предыдущую шторку свернули.
  useEffect(() => {
    setOptionsCollapsed(false)
    setSheetDragOffset(0)
  }, [optionsKey])

  useEffect(() => () => detachRecognition(true), [detachRecognition])

  const startSheetDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId)
    sheetDragStartRef.current = event.clientY
    sheetDragMovedRef.current = false
    setSheetDragOffset(0)
  }

  const moveSheetDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (sheetDragStartRef.current === null) return
    const delta = event.clientY - sheetDragStartRef.current
    if (Math.abs(delta) > 6) sheetDragMovedRef.current = true
    setSheetDragOffset(delta > 0 ? Math.min(delta, 120) : Math.max(delta, -120))
  }

  const endSheetDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (sheetDragStartRef.current === null) return
    const delta = event.clientY - sheetDragStartRef.current
    const moved = sheetDragMovedRef.current
    sheetDragStartRef.current = null
    sheetDragMovedRef.current = false
    setSheetDragOffset(0)

    if (delta > 28) {
      setOptionsCollapsed(true)
    } else if (delta < -28) {
      setOptionsCollapsed(false)
    } else if (!moved) {
      setOptionsCollapsed((value) => !value)
    }
  }

  const cancelSheetDrag = () => {
    sheetDragStartRef.current = null
    sheetDragMovedRef.current = false
    setSheetDragOffset(0)
  }

  return (
    <form
      onSubmit={submit}
      className="shrink-0 bg-white px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]"
    >
      {hasQuickOptions && (
        <div
          className="mb-2.5 overflow-hidden rounded-2xl border border-line bg-white transition-transform duration-150"
          style={{ transform: sheetDragOffset === 0 ? undefined : `translateY(${sheetDragOffset}px)` }}
        >
          <button
            type="button"
            aria-expanded={!optionsCollapsed}
            aria-controls="quick-options"
            onPointerDown={startSheetDrag}
            onPointerMove={moveSheetDrag}
            onPointerUp={endSheetDrag}
            onPointerCancel={cancelSheetDrag}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                setOptionsCollapsed((value) => !value)
              }
            }}
            className="flex w-full touch-none select-none flex-col items-center px-3 pt-2 pb-1.5 text-muted"
          >
            <span aria-hidden className="h-1.5 w-12 rounded-full bg-line" />
            <span className="mt-1 flex items-center gap-1.5 text-[12px] font-medium">
              {optionsCollapsed ? `Показать варианты (${options.length})` : 'Свернуть варианты'}
              <svg
                viewBox="0 0 24 24"
                className={`size-3.5 transition-transform ${optionsCollapsed ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                aria-hidden
              >
                <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </span>
          </button>

          <div
            id="quick-options"
            className={`grid transition-all duration-300 ${
              optionsCollapsed ? 'grid-rows-[0fr] opacity-0' : 'grid-rows-[1fr] opacity-100'
            }`}
          >
            <div className="overflow-hidden">
              <div className="px-2.5 pb-2.5">
                {showBodyMapButton && onOpenBodyMap !== undefined && (
                  <button
                    type="button"
                    onClick={() => {
                      stopSpeech()
                      onOpenBodyMap()
                    }}
                    className="mb-2.5 flex h-10 w-full items-center justify-center gap-1.5 rounded-xl border border-[#D1D5DB] bg-white text-[13px] font-medium text-ink transition-colors hover:bg-soft"
                  >
                    <span aria-hidden>🧍</span> Карта тела
                  </button>
                )}

                {options.length > 0 && (
                  <div className={grid ? 'grid grid-cols-2 gap-2' : 'flex flex-col gap-2'}>
                    {options.map((option, index) => {
                      const emoji = optionEmoji(option)
                      const fullRow = grid && options.length % 2 === 1 && index === options.length - 1
                      return (
                        <button
                          key={option}
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            stopSpeech()
                            setSpeechHint('')
                            setText('')
                            onAnswer(option)
                          }}
                          className={`flex min-h-11 min-w-0 items-center gap-1.5 rounded-xl bg-brand px-3 py-2.5 text-left leading-snug text-white transition-colors hover:bg-brand-hover disabled:opacity-60 ${
                            grid ? 'text-[14px]' : 'px-4 text-[15px]'
                          } ${fullRow ? 'col-span-2' : ''}`}
                        >
                          {emoji !== '' && (
                            <span aria-hidden className="shrink-0">
                              {emoji}
                            </span>
                          )}
                          <span className="min-w-0 font-medium whitespace-normal break-words">{option}</span>
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {allowFreeText && (
        <div>
          <div className="flex items-center gap-2">
            <input
              ref={inputRef}
              value={text}
              onChange={(event) => handleTextChange(event.target.value)}
              disabled={pending}
              placeholder={listening ? 'Говорите…' : 'Или опишите своими словами...'}
              aria-label="Свой ответ"
              className="h-11 min-w-0 flex-1 rounded-xl border border-[#D1D5DB] bg-white px-3.5 text-[15px] text-ink transition-colors placeholder:text-muted focus:border-brand focus:outline-none disabled:bg-soft"
            />

            {speechSupported && (
              <button
                type="button"
                onClick={toggleSpeech}
                disabled={pending}
                aria-label={listening ? 'Остановить запись' : 'Начать голосовой ввод'}
                aria-pressed={listening}
                className={`flex size-11 shrink-0 items-center justify-center rounded-xl transition-colors disabled:opacity-40 ${
                  listening
                    ? 'animate-pulse bg-red-600 text-white hover:bg-red-700'
                    : 'border border-[#D1D5DB] bg-white text-ink hover:bg-soft'
                }`}
              >
                <svg
                  viewBox="0 0 24 24"
                  className="size-5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden
                >
                  <rect x="9" y="2" width="6" height="12" rx="3" />
                  <path d="M5 10a7 7 0 0 0 14 0M12 17v4M8 21h8" />
                </svg>
              </button>
            )}

            <button
              type="submit"
              disabled={pending || text.trim() === ''}
              aria-label="Отправить ответ"
              className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-brand text-white transition-colors hover:bg-brand-hover disabled:opacity-40"
            >
              <svg
                viewBox="0 0 24 24"
                className="size-5"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden
              >
                <path d="M22 2 11 13M22 2l-7 20-4-9-9-4 20-7Z" />
              </svg>
            </button>
          </div>

          {(listening || speechHint !== '') && (
            <p className="mt-1.5 px-1 text-[12px] leading-snug text-muted" role="status" aria-live="polite">
              {speechHint || 'Говорите…'}
            </p>
          )}
        </div>
      )}
    </form>
  )
}
