import { useEffect, useState, type ReactNode } from 'react'

type BodySide = 'front' | 'back'

type EllipseShape = {
  kind: 'ellipse'
  cx: number
  cy: number
  rx: number
  ry: number
}

type PathShape = {
  kind: 'path'
  d: string
  width: number
}

type Shape = EllipseShape | PathShape

type Zone = {
  label: string
  shapes: Shape[]
  heart?: boolean
}

const BODY = '#DDE3EA'

const TORSO = {
  male: {
    chest: 'M98 84 Q120 78 142 84 L152 112 L150 162 Q120 170 90 162 L88 112 Z',
    abdomen: 'M90 162 Q120 170 150 162 L148 204 Q120 214 92 204 Z',
    pelvis: 'M92 204 Q120 214 148 204 L150 232 Q120 244 90 232 Z',
  },
  female: {
    chest: 'M101 84 Q120 79 139 84 L148 110 L146 156 Q120 164 94 156 L92 110 Z',
    abdomen: 'M94 156 Q120 164 146 156 L143 198 Q120 208 97 198 Z',
    pelvis: 'M97 198 Q120 208 143 198 L150 230 Q120 244 90 230 Z',
  },
}

const ARM = {
  upperLeft: 'M80 108 L66 164',
  upperRight: 'M160 108 L174 164',
  forearmLeft: 'M63 176 L57 230',
  forearmRight: 'M177 176 L183 230',
}

const LEG = {
  thighLeft: 'M102 236 L94 320',
  thighRight: 'M138 236 L146 320',
  shinLeft: 'M94 332 L90 410',
  shinRight: 'M146 332 L150 410',
}

function shapes(...items: Shape[]): Shape[] {
  return items
}

function path(d: string, width = 18): PathShape {
  return { kind: 'path', d, width }
}

function ellipse(cx: number, cy: number, rx: number, ry: number): EllipseShape {
  return { kind: 'ellipse', cx, cy, rx, ry }
}

const FRONT_ZONES: Zone[] = [
  { label: 'Голова', shapes: shapes(ellipse(120, 42, 21, 27)) },
  { label: 'Шея', shapes: shapes(path('M120 66 L120 88', 15)) },
  {
    label: 'Грудная клетка',
    shapes: shapes(path(TORSO.male.chest, 0)),
  },
  { label: 'Живот', shapes: shapes(path(TORSO.male.abdomen, 0)) },
  { label: 'Таз', shapes: shapes(path(TORSO.male.pelvis, 0)) },
  {
    label: 'Плечи',
    shapes: shapes(ellipse(80, 102, 12, 10), ellipse(160, 102, 12, 10)),
  },
  { label: 'Руки', shapes: shapes(path(ARM.upperLeft), path(ARM.upperRight)) },
  {
    label: 'Локти',
    shapes: shapes(ellipse(65, 169, 10, 10), ellipse(175, 169, 10, 10)),
  },
  {
    label: 'Предплечья',
    shapes: shapes(path(ARM.forearmLeft), path(ARM.forearmRight)),
  },
  {
    label: 'Кисти',
    shapes: shapes(ellipse(56, 245, 10, 13), ellipse(184, 245, 10, 13)),
  },
  {
    label: 'Бёдра',
    shapes: shapes(path(LEG.thighLeft, 20), path(LEG.thighRight, 20)),
  },
  {
    label: 'Колени',
    shapes: shapes(ellipse(94, 325, 10, 10), ellipse(146, 325, 10, 10)),
  },
  {
    label: 'Голени',
    shapes: shapes(path(LEG.shinLeft), path(LEG.shinRight)),
  },
  {
    label: 'Сердце / область сердца',
    shapes: shapes(ellipse(145, 122, 15, 15)),
    heart: true,
  },
]

const BACK_ZONES: Zone[] = [
  { label: 'Голова', shapes: shapes(ellipse(120, 42, 21, 27)) },
  { label: 'Шея', shapes: shapes(path('M120 66 L120 88', 15)) },
  {
    label: 'Спина / поясница',
    shapes: shapes(path(TORSO.male.chest, 0), path(TORSO.male.abdomen, 0)),
  },
  { label: 'Таз', shapes: shapes(path(TORSO.male.pelvis, 0)) },
  ...FRONT_ZONES.filter((zone) =>
    ['Плечи', 'Руки', 'Локти', 'Предплечья', 'Кисти', 'Бёдра', 'Колени', 'Голени'].includes(zone.label),
  ),
]

function torsoFor(female: boolean) {
  return female ? TORSO.female : TORSO.male
}

function baseBody(female: boolean): ReactNode {
  const torso = torsoFor(female)
  return (
    <g strokeLinecap="round" strokeLinejoin="round">
      <g fill={BODY} stroke="none">
        <ellipse cx={120} cy={42} rx={21} ry={27} />
        <path d={torso.chest} />
        <path d={torso.abdomen} />
        <path d={torso.pelvis} />
        <ellipse cx={56} cy={245} rx={9} ry={12} />
        <ellipse cx={184} cy={245} rx={9} ry={12} />
        <ellipse cx={88} cy={418} rx={12} ry={5} />
        <ellipse cx={152} cy={418} rx={12} ry={5} />
      </g>

      <g fill="none" stroke={BODY} strokeWidth={13}>
        <path d="M120 66 L120 88" strokeWidth={14} />
        <path d="M89 111 L79 106" />
        <path d="M151 111 L161 106" />
        <path d={ARM.upperLeft} />
        <path d={ARM.upperRight} />
        <path d={ARM.forearmLeft} />
        <path d={ARM.forearmRight} />
        <path d={LEG.thighLeft} strokeWidth={14} />
        <path d={LEG.thighRight} strokeWidth={14} />
        <path d={LEG.shinLeft} />
        <path d={LEG.shinRight} />
      </g>

      <g fill={BODY} stroke="none">
        <circle cx={79} cy={104} r={10} />
        <circle cx={161} cy={104} r={10} />
        <circle cx={65} cy={169} r={7} />
        <circle cx={175} cy={169} r={7} />
        <circle cx={94} cy={325} r={7} />
        <circle cx={146} cy={325} r={7} />
      </g>
    </g>
  )
}

function zoneContent(zone: Zone, female: boolean): ReactNode {
  const torso = torsoFor(female)
  return (
    <g>
      {zone.shapes.map((shape, index) => {
        if (shape.kind === 'ellipse') {
          return (
            <ellipse
              key={`${zone.label}-${index}`}
              cx={shape.cx}
              cy={shape.cy}
              rx={shape.rx}
              ry={shape.ry}
              className={`pointer-events-auto fill-transparent transition-colors ${
                zone.heart ? 'hover:fill-[#FCA5A5]/50' : 'hover:fill-[#BFDBFE]/70'
              }`}
            />
          )
        }

        const d =
          zone.label === 'Грудная клетка' || (zone.label === 'Спина / поясница' && index === 0)
            ? torso.chest
            : zone.label === 'Живот' || (zone.label === 'Спина / поясница' && index === 1)
              ? torso.abdomen
              : zone.label === 'Таз'
                ? torso.pelvis
                : shape.d

        if (zone.label === 'Грудная клетка' || zone.label === 'Спина / поясница' || zone.label === 'Живот' || zone.label === 'Таз') {
          return (
            <path
              key={`${zone.label}-${index}`}
              d={d}
              className="pointer-events-auto fill-transparent transition-colors hover:fill-[#BFDBFE]/70"
            />
          )
        }

        return (
          <path
            key={`${zone.label}-${index}`}
            d={d}
            fill="none"
            stroke="transparent"
            strokeWidth={shape.width}
            strokeLinecap="round"
            className="pointer-events-auto transition-colors hover:stroke-[#BFDBFE]/70"
          />
        )
      })}
    </g>
  )
}

type BodyMapModalProps = {
  open: boolean
  sex: number
  onClose: () => void
  onSelect: (location: string) => void
}

export function BodyMapModal({ open, sex, onClose, onSelect }: BodyMapModalProps) {
  const [side, setSide] = useState<BodySide>('front')
  const [hovered, setHovered] = useState('')

  useEffect(() => {
    if (open) {
      setSide('front')
      setHovered('')
    }
  }, [open])

  if (!open) return null

  const female = sex === 2
  const zones = side === 'front' ? FRONT_ZONES : BACK_ZONES

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 sm:items-center sm:p-4"
      role="presentation"
      onClick={onClose}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="body-map-title"
        className="max-h-[92dvh] w-full max-w-md overflow-y-auto rounded-t-2xl bg-white p-4 sm:rounded-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 id="body-map-title" className="text-lg font-semibold text-ink">
              🧍 Где беспокоит?
            </h2>
            <p className="mt-1 text-[13px] leading-relaxed text-muted">
              Нажмите на нужную зону тела.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть карту тела"
            className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-soft text-ink transition-colors hover:bg-line"
          >
            <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth={2} aria-hidden>
              <path d="m6 6 12 12M18 6 6 18" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        <div className="mt-3 flex rounded-xl bg-soft p-1">
          {(['front', 'back'] as const).map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => {
                setSide(value)
                setHovered('')
              }}
              aria-pressed={side === value}
              className={`h-9 flex-1 rounded-lg text-[13px] font-medium transition-colors ${
                side === value ? 'bg-brand text-white' : 'text-ink'
              }`}
            >
              {value === 'front' ? 'Спереди' : 'Сзади'}
            </button>
          ))}
        </div>

        <p className="mt-2 min-h-5 text-center text-[12px] text-muted" role="status" aria-live="polite">
          {hovered === '' ? 'Наведите на зону' : hovered}
        </p>

        <div className="flex justify-center rounded-2xl bg-canvas p-2">
          <svg
            viewBox="0 0 240 440"
            className="h-80 w-auto"
            role="img"
            aria-label={`Схема тела, ${side === 'front' ? 'спереди' : 'сзади'}`}
          >
            {baseBody(female)}
            {zones.map((zone) => (
              <g
                key={`${side}-${zone.label}`}
                role="button"
                tabIndex={0}
                aria-label={zone.label}
                className="cursor-pointer outline-none"
                onMouseEnter={() => setHovered(zone.label)}
                onMouseLeave={() => setHovered('')}
                onFocus={() => setHovered(zone.label)}
                onBlur={() => setHovered('')}
                onPointerDown={(event) => {
                  event.preventDefault()
                  onSelect(zone.label)
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    onSelect(zone.label)
                  }
                }}
              >
                <title>{zone.label}</title>
                {zoneContent(zone, female)}
                {zone.heart && (
                  <>
                    <circle cx={145} cy={122} r={12} className="fill-[#FEE2E2]" />
                    <path
                      d="M145 127 C141 123 139 119 142 117 C145 115 146 118 145 120 C144 118 145 115 148 117 C151 119 149 123 145 127 Z"
                      className="fill-[#DC2626]"
                    />
                  </>
                )}
              </g>
            ))}
          </svg>
        </div>
      </section>
    </div>
  )
}
