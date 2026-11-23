import { useState, type FormEvent } from 'react'
import { optionEmoji } from '../lib/format'

type ComposerProps = {
  options: string[]
  allowFreeText: boolean
  pending: boolean
  onAnswer: (answer: string) => void
}

export function Composer({ options, allowFreeText, pending, onAnswer }: ComposerProps) {
  const [text, setText] = useState('')
  // Компактная сетка для коротких/средних подписей; очень длинные варианты остаются в одну колонку.
  const grid = options.length > 4 && options.every((option) => option.length <= 32)

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const value = text.trim()
    if (value === '' || pending) return
    onAnswer(value)
    setText('')
  }

  return (
    <form
      onSubmit={submit}
      className="shrink-0 bg-white px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]"
    >
      {options.length > 0 && (
        <div className={grid ? 'mb-2.5 grid grid-cols-2 gap-2' : 'mb-2.5 flex flex-col gap-2'}>
          {options.map((option, index) => {
            const emoji = optionEmoji(option)
            const fullRow = grid && options.length % 2 === 1 && index === options.length - 1
            return (
              <button
                key={option}
                type="button"
                disabled={pending}
                onClick={() => {
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

      {allowFreeText && (
        <div className="flex items-center gap-2">
          <input
            value={text}
            onChange={(event) => setText(event.target.value)}
            disabled={pending}
            placeholder="Или опишите своими словами..."
            aria-label="Свой ответ"
            className="h-11 min-w-0 flex-1 rounded-xl border border-[#D1D5DB] bg-white px-3.5 text-[15px] text-ink transition-colors placeholder:text-muted focus:border-brand focus:outline-none disabled:bg-soft"
          />
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
      )}
    </form>
  )
}
