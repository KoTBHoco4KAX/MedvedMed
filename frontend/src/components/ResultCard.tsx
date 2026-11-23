import type { FinalCard } from '../types'
import { urgencyEmoji, urgencyLabel, urgencyTextClass, urgencyTone } from '../lib/format'

type ResultCardProps = {
  card: FinalCard
  onRestart: () => void
}

export function ResultCard({ card, onRestart }: ResultCardProps) {
  const tone = urgencyTone(card.urgency)
  const emergency = tone === 'emergency'

  return (
    <article
      data-card="result"
      className={`rounded-2xl bg-white p-5 ${emergency ? 'border-2 border-red-600' : ''}`}
    >
      {emergency ? (
        <h2 className="text-lg leading-snug font-semibold text-red-700">
          Требуется экстренная медицинская помощь — вызовите скорую (103/112)
        </h2>
      ) : (
        <span className={`inline-flex items-center gap-1.5 text-[12px] font-medium ${urgencyTextClass(tone)}`}>
          <span aria-hidden>{urgencyEmoji(tone)}</span>
          {urgencyLabel(card.urgency)}
        </span>
      )}

      <div className="mt-4">
        <p className="text-[11px] font-medium tracking-wide text-muted uppercase">Врач</p>
        <h2 className="mt-1 text-xl leading-snug font-semibold text-ink">{card.specialty}</h2>
      </div>

      <div className="mt-3">
        <p className="text-[11px] font-medium tracking-wide text-muted uppercase">Куда обратиться</p>
        <p className="mt-1 text-[15px] leading-snug font-medium text-ink">{card.place}</p>
      </div>

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="Обоснование" text={card.reasoning} />

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="По оплате (ОМС/ДМС/платно)" text={card.paymentNote} />

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="Что подготовить к приёму" text={card.preparationTips} />

      <p className="mt-5 text-[13px] leading-relaxed font-medium text-amber-800">
        ⚠️ При ухудшении состояния немедленно вызывайте скорую помощь (103/112)
      </p>

      {emergency && (
        <div className="mt-4 grid grid-cols-2 gap-2">
          <a
            href="tel:103"
            className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
          >
            Позвонить 103
          </a>
          <a
            href="tel:112"
            className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
          >
            Позвонить 112
          </a>
        </div>
      )}

      <button
        type="button"
        onClick={onRestart}
        className="mt-4 h-11 w-full rounded-xl bg-brand text-[15px] font-medium text-white transition-colors hover:bg-brand-hover"
      >
        Пройти опрос заново
      </button>
    </article>
  )
}

function Section({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <h3 className="text-[13px] font-semibold text-ink">{title}</h3>
      <p className="mt-1.5 whitespace-pre-line text-[14px] leading-relaxed text-[#374151]">{text}</p>
    </section>
  )
}
