/// <reference types="vite/client" />

interface SpeechRecognitionAlternativeLike {
  transcript: string
  confidence: number
}

interface SpeechRecognitionResultLike {
  readonly isFinal: boolean
  readonly length: number
  readonly [index: number]: SpeechRecognitionAlternativeLike
}

interface SpeechRecognitionResultListLike {
  readonly length: number
  readonly [index: number]: SpeechRecognitionResultLike
}

interface SpeechRecognitionEventLike extends Event {
  readonly resultIndex: number
  readonly results: SpeechRecognitionResultListLike
}

interface SpeechRecognitionErrorEventLike extends Event {
  readonly error: string
  readonly message: string
}

interface SpeechRecognitionLike extends EventTarget {
  lang: string
  continuous: boolean
  interimResults: boolean
  maxAlternatives: number
  start: () => void
  stop: () => void
  abort: () => void
  onstart: ((this: SpeechRecognitionLike, event: Event) => void) | null
  onresult: ((this: SpeechRecognitionLike, event: SpeechRecognitionEventLike) => void) | null
  onerror: ((this: SpeechRecognitionLike, event: SpeechRecognitionErrorEventLike) => void) | null
  onend: ((this: SpeechRecognitionLike, event: Event) => void) | null
}

interface SpeechRecognitionConstructorLike {
  new (): SpeechRecognitionLike
}

interface Window {
  SpeechRecognition?: SpeechRecognitionConstructorLike
  webkitSpeechRecognition?: SpeechRecognitionConstructorLike
}
