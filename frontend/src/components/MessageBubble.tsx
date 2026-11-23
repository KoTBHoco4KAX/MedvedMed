import type { ChatMessage } from '../types'

type MessageBubbleProps = {
  message: ChatMessage
}

export function MessageBubble({ message }: MessageBubbleProps) {
  if (message.role === 'user') {
    return (
      <div className="ml-auto max-w-[85%] rounded-xl bg-brand px-4 py-2.5 text-[15px] leading-relaxed text-white">
        {message.text}
      </div>
    )
  }

  return (
    <div className="max-w-[88%] rounded-xl bg-white px-4 py-3 text-[15px] leading-relaxed text-ink">
      {message.text}
    </div>
  )
}

type TypingBubbleProps = {
  label: string
}

export function TypingBubble({ label }: TypingBubbleProps) {
  return (
    <div
      className="max-w-[88%] rounded-xl bg-white px-4 py-3"
      role="status"
      aria-live="polite"
    >
      <span className="flex items-center gap-1.5">
        <span className="size-1.5 animate-bounce rounded-full bg-[#9CA3AF] [animation-delay:-0.3s]" />
        <span className="size-1.5 animate-bounce rounded-full bg-[#9CA3AF] [animation-delay:-0.15s]" />
        <span className="size-1.5 animate-bounce rounded-full bg-[#9CA3AF]" />
        <span className="ml-1.5 text-[13px] text-muted">{label}</span>
      </span>
    </div>
  )
}
