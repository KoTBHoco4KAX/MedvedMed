import { useState } from 'react'

type GuideItem = {
  id: string
  title: string
  action: string
}

const ACTION_CHEST =
  'Сядьте полусидя с опорой под спину, расстегните тесную одежду, откройте окно для притока воздуха, не ходите и не нагружайте себя, не ешьте и не пейте.'
const ACTION_DYSPNEA =
  'Сядьте, слегка наклонившись вперёд с опорой на руки, расстегните воротник и пояс, откройте окно, старайтесь дышать медленно и ровно, не ложитесь горизонтально.'
const ACTION_SYNCOPE =
  'Если человек без сознания, но дышит — уложите его на бок (устойчивое боковое положение), расстегните тесную одежду, обеспечьте приток воздуха, не пытайтесь поить водой или поднимать.'
const ACTION_SEIZURE =
  'Уберите рядом острые и твёрдые предметы, подложите под голову что-то мягкое, поверните голову набок; не разжимайте челюсти, не вставляйте предметы в рот и не удерживайте человека силой.'
const ACTION_BLEEDING =
  'При наружном кровотечении плотно прижмите рану чистой тканью или бинтом и по возможности приподнимите повреждённую конечность; при рвоте с кровью или чёрном стуле уложите человека, не давайте еду и питьё.'
const ACTION_STROKE =
  'Уложите человека с приподнятой на 30° головой и плечами (при тошноте или сонливости — на бок), расстегните тесную одежду, не давайте воду, еду и лекарства, запомните точное время начала симптомов.'
const ACTION_ANAPHYLAXIS =
  'Немедленно прекратите контакт с возможным аллергеном, уложите человека на спину и приподнимите ноги (при одышке — усадите полусидя), расстегните тесную одежду, откройте окно.'
const ACTION_ABDOMEN =
  'Лягте в удобное положение (обычно на бок с подтянутыми к животу коленями), не прикладывайте грелку к животу, не ешьте, не пейте и не принимайте обезболивающие до осмотра врача (они могут смазать картину).'
const ACTION_CHILD_FEVER =
  'Снимите с ребёнка лишнюю одежду и не укутывайте, проветрите комнату, предлагайте прохладную воду маленькими глотками, не обтирайте спиртом или уксусом и не давайте препараты без назначения врача.'
const ACTION_HEAD_TRAUMA =
  'Обеспечьте полный покой, уложите человека (при тошноте/рвоте — строго на бок), не оставляйте одного, не давайте еду, воду и седативные средства.'

const GUIDE_ITEMS: GuideItem[] = [
  { id: 'chest', title: 'Боль или сдавливание в груди', action: ACTION_CHEST },
  { id: 'dyspnea', title: 'Одышка в покое или удушье', action: ACTION_DYSPNEA },
  { id: 'syncope', title: 'Потеря сознания или обморок', action: ACTION_SYNCOPE },
  { id: 'seizure', title: 'Судороги', action: ACTION_SEIZURE },
  { id: 'bleeding', title: 'Сильное кровотечение', action: ACTION_BLEEDING },
  { id: 'stroke', title: 'Признаки инсульта FAST', action: ACTION_STROKE },
  { id: 'anaphylaxis', title: 'Анафилаксия или отёк Квинке', action: ACTION_ANAPHYLAXIS },
  { id: 'abdomen', title: 'Острая боль в животе', action: ACTION_ABDOMEN },
  { id: 'child_fever', title: 'Температура выше 39,5 °C у ребёнка', action: ACTION_CHILD_FEVER },
  { id: 'head_trauma', title: 'Травма головы с потерей сознания или рвотой', action: ACTION_HEAD_TRAUMA },
]

type EmergencyGuideModalProps = {
  open: boolean
  onClose: () => void
}

export function EmergencyGuideModal({ open, onClose }: EmergencyGuideModalProps) {
  const [openId, setOpenId] = useState<string | null>(null)
  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 sm:items-center sm:p-4"
      role="presentation"
      onClick={onClose}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="emergency-guide-title"
        className="max-h-[92dvh] w-full max-w-md overflow-y-auto rounded-t-2xl bg-white p-4 sm:rounded-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 id="emergency-guide-title" className="text-lg font-semibold text-ink">
              ⚠️ Опасные симптомы
            </h2>
            <p className="mt-1 text-[13px] leading-relaxed text-muted">
              Нажмите на симптом, чтобы увидеть первую помощь до приезда скорой.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть памятку"
            className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-soft text-ink transition-colors hover:bg-line"
          >
            <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth={2} aria-hidden>
              <path d="m6 6 12 12M18 6 6 18" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        <ul className="mt-3 flex flex-col divide-y divide-line">
          {GUIDE_ITEMS.map((item) => {
            const expanded = openId === item.id
            return (
              <li key={item.id}>
                <button
                  type="button"
                  onClick={() => setOpenId(expanded ? null : item.id)}
                  aria-expanded={expanded}
                  className="flex w-full items-center justify-between gap-3 py-3 text-left text-[14px] font-medium text-ink"
                >
                  <span>{item.title}</span>
                  <svg
                    viewBox="0 0 24 24"
                    className={`size-4 shrink-0 text-muted transition-transform ${expanded ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2}
                    aria-hidden
                  >
                    <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </button>

                <div
                  className={`grid transition-all duration-300 ease-out ${
                    expanded ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'
                  }`}
                >
                  <div className="overflow-hidden" aria-hidden={!expanded}>
                    <div className="pb-4">
                      <h3 className="text-[12px] font-semibold tracking-wide text-red-700 uppercase">
                        Что делать до приезда скорой
                      </h3>
                      <p className="mt-1.5 text-[14px] leading-relaxed text-[#374151]">{item.action}</p>
                      <div className="mt-3 grid grid-cols-2 gap-2">
                        <a
                          href="tel:103"
                          tabIndex={expanded ? 0 : -1}
                          className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
                        >
                          Позвонить 103
                        </a>
                        <a
                          href="tel:112"
                          tabIndex={expanded ? 0 : -1}
                          className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
                        >
                          Позвонить 112
                        </a>
                      </div>
                    </div>
                  </div>
                </div>
              </li>
            )
          })}
        </ul>
      </section>
    </div>
  )
}
