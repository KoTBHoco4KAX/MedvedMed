export type AuthSession = {
  sessionToken: string
  firstName: string
  age: number
  sex: number
}

export type FinalCard = {
  specialty: string
  urgency: string
  place: string
  reasoning: string
  paymentNote: string
  preparationTips: string
}

export type ChatMessageRole = 'bot' | 'user' | 'emergency' | 'card'

export type ChatMessage = {
  role: ChatMessageRole
  step: string
  text: string
  at: string
  card?: FinalCard
}

export type ChatProgress = {
  current: number
  total: number
}

export type ChatState = {
  sessionToken: string
  firstName: string
  age: number
  sex: number
  step: string
  question: string
  options: string[]
  allowFreeText: boolean
  canGoBack: boolean
  progress: ChatProgress
  messages: ChatMessage[]
  isEmergency: boolean
  emergencyReason?: string
  emergencyInstruction?: string
  completed: boolean
  card?: FinalCard
}

export type HistoryAnswer = {
  question: string
  answer: string
}

export type HistoryItem = {
  id: number
  createdAt: string
  card: FinalCard
  answers: HistoryAnswer[]
}
