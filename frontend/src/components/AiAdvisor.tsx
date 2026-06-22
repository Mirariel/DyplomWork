import { useState, useRef, useEffect } from 'react'
import { Bot, X, Send, Loader2 } from 'lucide-react'
import { askAI, type AIMessage } from '../api'

const QUICK = [
  'What is my win rate?',
  'Which symbols are most profitable?',
  'What are my biggest mistakes?',
  'Give me a trading tip',
]

export default function AiAdvisor() {
  const [open, setOpen] = useState(false)
  const [messages, setMessages] = useState<AIMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, open])

  const send = async (text: string) => {
    if (!text.trim() || loading) return
    setInput('')
    setError('')

    const newMessages: AIMessage[] = [...messages, { role: 'user', content: text }]
    setMessages(newMessages)
    setLoading(true)

    try {
      const { reply } = await askAI(text, messages)
      setMessages([...newMessages, { role: 'assistant', content: reply }])
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'AI advisor unavailable'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      {/* Floating button */}
      <button
        onClick={() => setOpen((o) => !o)}
        className={`fixed bottom-6 right-6 z-40 w-12 h-12 rounded-full shadow-lg flex items-center justify-center transition-all ${
          open ? 'bg-slate-700 text-slate-300' : 'bg-blue-600 hover:bg-blue-500 text-white'
        }`}
        title="AI Trading Advisor"
      >
        {open ? <X size={20} /> : <Bot size={22} />}
      </button>

      {/* Chat panel */}
      {open && (
        <div className="fixed bottom-20 right-6 z-40 w-80 sm:w-96 bg-slate-800 border border-slate-700 rounded-2xl shadow-2xl flex flex-col overflow-hidden max-h-[70vh]">
          {/* Header */}
          <div className="flex items-center gap-2 px-4 py-3 border-b border-slate-700 bg-slate-800/90">
            <Bot size={18} className="text-blue-400" />
            <div className="flex-1">
              <p className="text-sm font-semibold text-white">AI Trading Advisor</p>
              <p className="text-xs text-slate-500">Powered by Claude</p>
            </div>
            <button onClick={() => setOpen(false)} className="text-slate-500 hover:text-white transition-colors">
              <X size={16} />
            </button>
          </div>

          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-4 space-y-3 min-h-0">
            {messages.length === 0 && (
              <div className="text-center py-4">
                <Bot size={32} className="mx-auto text-slate-600 mb-2" />
                <p className="text-sm text-slate-400">Ask me about your trading performance</p>
                <div className="mt-3 space-y-1.5">
                  {QUICK.map((q) => (
                    <button
                      key={q}
                      onClick={() => void send(q)}
                      className="block w-full text-left px-3 py-2 text-xs bg-slate-700/60 hover:bg-slate-700 text-slate-300 rounded-lg transition-colors"
                    >
                      {q}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {messages.map((m, i) => (
              <div key={i} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                <div
                  className={`max-w-[85%] px-3 py-2 rounded-xl text-sm whitespace-pre-wrap ${
                    m.role === 'user'
                      ? 'bg-blue-600 text-white rounded-br-sm'
                      : 'bg-slate-700 text-slate-200 rounded-bl-sm'
                  }`}
                >
                  {m.content}
                </div>
              </div>
            ))}

            {loading && (
              <div className="flex justify-start">
                <div className="bg-slate-700 px-3 py-2 rounded-xl rounded-bl-sm">
                  <Loader2 size={16} className="text-slate-400 animate-spin" />
                </div>
              </div>
            )}

            {error && (
              <p className="text-xs text-red-400 text-center py-1">{error}</p>
            )}

            <div ref={bottomRef} />
          </div>

          {/* Input */}
          <div className="px-3 py-3 border-t border-slate-700">
            <div className="flex gap-2">
              <input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void send(input) } }}
                placeholder="Ask about your trades…"
                disabled={loading}
                className="flex-1 bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 disabled:opacity-50"
              />
              <button
                onClick={() => void send(input)}
                disabled={loading || !input.trim()}
                className="w-9 h-9 flex items-center justify-center bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white rounded-lg transition-colors flex-shrink-0"
              >
                <Send size={15} />
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
