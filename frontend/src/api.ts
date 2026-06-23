import axios from 'axios'

// ─── Axios instance ──────────────────────────────────────────────────────────

const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('tt_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// ─── Types ───────────────────────────────────────────────────────────────────

export interface User {
  id: number
  username: string
  email: string
  created_at: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface UserAsset {
  id: number
  user_id: number
  asset_id: number
  exchange: string
  symbol: string
  quantity: number
  avg_buy_price: number
  current_price: number
  manually_set: boolean
  updated_at: string
}

export interface Position {
  id: number
  user_id: number
  asset_id: number
  exchange: string
  symbol: string
  side: string
  margin_mode: string
  entry_price: number
  mark_price: number
  quantity: number
  pnl: number
  leverage: string
  liq_price: number
  margin: number
  trade_size: number
  comment: string | null
  updated_at: string
}

export interface HistoryEntry {
  id: number
  user_id: number
  symbol: string
  exchange: string
  side: string        // LONG | SHORT
  margin_mode: string
  leverage: string
  entry_price: number
  exit_price: number
  quantity: number
  realized_pnl: number
  fee: number
  max_size: number    // position size in USD (notionalUsd from OKX, or qty×entry fallback)
  opened_at: string | null
  closed_at: string | null
  comment: string | null
  created_at: string
}

export interface HistoryResponse {
  data: HistoryEntry[]
  total: number
  limit: number
  offset: number
}

export interface Credential {
  id: number
  user_id: number
  exchange: string
  label: string
  api_key_hint: string
  has_passphrase: boolean
  is_active: boolean
  last_sync_at: string | null
  last_sync_error: string | null
  created_at: string
}

export interface PortfolioSummary {
  assets: UserAsset[]
  positions: Position[]
  history: HistoryEntry[]
  total_value: number
}

export interface AddCredentialPayload {
  exchange: string
  label: string
  api_key: string
  api_secret: string
  passphrase?: string
}

export interface Order {
  id: number
  user_id: number
  exchange: string
  symbol: string
  side: string
  category: string
  type: string
  quantity: number
  price: number
  status: string
  exchange_order_id: string
  created_at: string
  updated_at: string
}

export interface PlaceOrderPayload {
  exchange: string
  symbol: string
  side: string
  category: string
  type: string
  quantity: number
  price?: number
}

export interface SmartOrder {
  id: number
  user_id: number
  exchange: string
  symbol: string
  side: string
  category: string
  quantity: number
  order_type: string
  trigger_price: number
  callback_rate: number
  status: string
  created_at: string
  updated_at: string
}

export interface CreateSmartOrderPayload {
  exchange: string
  symbol: string
  side: string
  category: string
  quantity: number
  order_type: string
  trigger_price?: number
  callback_rate?: number
}

export interface Bot {
  id: number
  user_id: number
  exchange: string
  symbol: string
  category: string
  status: string
  price_low: number
  price_high: number
  grids: number
  qty_per_grid: number
  total_profit: number
  created_at: string
  updated_at: string
  grid_levels?: GridLevel[]
}

export interface GridLevel {
  id: number
  bot_id: number
  price: number
  side: string
  status: string
  order_id: string
}

export interface CreateBotPayload {
  exchange: string
  symbol: string
  category: string
  price_low: number
  price_high: number
  grids: number
  qty_per_grid: number
}

export interface TradeSummary {
  total_trades: number
  winning_trades: number
  losing_trades: number
  winrate: number
  total_realized_pnl: number
  avg_pnl: number
  best_trade: number
  worst_trade: number
  avg_win: number
  avg_loss: number
  profit_factor: number
  total_fees: number
}

export interface CoinPerformance {
  symbol: string
  exchange: string
  total_trades: number
  winning_trades: number
  winrate: number
  total_pnl: number
  avg_pnl: number
  best_trade: number
  worst_trade: number
}

export interface Snapshot {
  id: number
  user_id: number
  total_value: number
  spot_value: number
  futures_pnl: number
  snapshot_at: string  // ISO datetime e.g. "2026-06-23T14:00:00Z"
}

export interface ChartPoint {
  recorded_at: string  // ISO datetime, 15-min bucketed
  value: number
}

// LivePosition — position data from WS broadcast (not from DB)
export interface LivePosition {
  symbol: string
  side: string
  exchange: string
  margin_mode: string
  leverage: string
  entry_price: number
  mark_price: number
  liq_price: number
  quantity: number
  trade_size: number
  margin: number
  pnl: number
  pnl_pct: number
}

export interface ArbitrageOpportunity {
  symbol: string
  buy_exchange: string
  buy_price: number
  sell_exchange: string
  sell_price: number
  spread_usd: number
  spread_pct: number
}

export interface DCABot {
  id: number
  user_id: number
  exchange: string
  symbol: string
  category: string
  amount_usd: number
  interval_hours: number
  status: string
  total_invested: number
  total_quantity: number
  next_buy_at: string
  created_at: string
  updated_at: string
}

export interface CreateDCAPayload {
  exchange: string
  symbol: string
  category: string
  amount_usd: number
  interval_hours: number
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

export const login = (login: string, password: string) =>
  api.post<AuthResponse>('/auth/login', { login, password }).then((r) => r.data)

export const register = (username: string, email: string, password: string) =>
  api.post<AuthResponse>('/auth/register', { username, email, password }).then((r) => r.data)

export const logout = () =>
  api.post('/auth/logout').then((r) => r.data)

export const me = () =>
  api.get<User>('/auth/me').then((r) => r.data)

export const updateProfile = (username: string, email: string) =>
  api.patch<User>('/user/profile', { username, email }).then((r) => r.data)

export const changePassword = (current_password: string, new_password: string) =>
  api.patch('/user/password', { current_password, new_password }).then((r) => r.data)

// ─── Portfolio ────────────────────────────────────────────────────────────────

export const getPortfolio = () =>
  api.get<PortfolioSummary>('/portfolio').then((r) => r.data)

export const getHistory = (limit = 15, offset = 0, from?: string, to?: string) =>
  api.get<HistoryResponse>('/portfolio/history', {
    params: { limit, offset, ...(from ? { from } : {}), ...(to ? { to } : {}) },
  }).then((r) => r.data)

export const getCredentials = () =>
  api.get<Credential[]>('/portfolio/credentials').then((r) => r.data)

export const addCredential = (data: AddCredentialPayload) =>
  api.post<Credential>('/portfolio/credentials', data).then((r) => r.data)

export const deleteCredential = (id: number) =>
  api.delete(`/portfolio/credentials/${id}`).then((r) => r.data)

export const updatePositionComment = (id: number, comment: string) =>
  api.patch<Position>(`/positions/${id}/comment`, { comment }).then((r) => r.data)

export const updateHistoryComment = (id: number, comment: string) =>
  api.patch<HistoryEntry>(`/history/${id}/comment`, { comment }).then((r) => r.data)

// ─── Sync ─────────────────────────────────────────────────────────────────────

export const fullSync = () =>
  api.post('/sync/full').then((r) => r.data)

export const syncPositions = () =>
  api.post('/sync/positions').then((r) => r.data)

export const syncHistory = (days = 30) =>
  api.post('/sync/history', null, { params: { days } }).then((r) => r.data)

export const updatePrices = () =>
  api.get('/sync/prices').then((r) => r.data)

// ─── Orders ───────────────────────────────────────────────────────────────────

export const placeOrder = (data: PlaceOrderPayload) =>
  api.post<Order>('/orders', data).then((r) => r.data)

export const listOrders = (status?: string) =>
  api.get<Order[]>('/orders', { params: status ? { status } : {} }).then((r) => r.data)

export const getOrder = (id: number) =>
  api.get<Order>(`/orders/${id}`).then((r) => r.data)

export const cancelOrder = (id: number) =>
  api.delete(`/orders/${id}`).then((r) => r.data)

// ─── Smart Orders ─────────────────────────────────────────────────────────────

export const createSmartOrder = (data: CreateSmartOrderPayload) =>
  api.post<SmartOrder>('/smart-orders', data).then((r) => r.data)

export const listSmartOrders = () =>
  api.get<SmartOrder[]>('/smart-orders').then((r) => r.data)

export const getSmartOrder = (id: number) =>
  api.get<SmartOrder>(`/smart-orders/${id}`).then((r) => r.data)

export const cancelSmartOrder = (id: number) =>
  api.delete(`/smart-orders/${id}`).then((r) => r.data)

// ─── Bots ─────────────────────────────────────────────────────────────────────

export const createBot = (data: CreateBotPayload) =>
  api.post<Bot>('/bots', data).then((r) => r.data)

export const listBots = () =>
  api.get<Bot[]>('/bots').then((r) => r.data)

export const getBot = (id: number) =>
  api.get<Bot>(`/bots/${id}`).then((r) => r.data)

export const startBot = (id: number) =>
  api.post(`/bots/${id}/start`).then((r) => r.data)

export const stopBot = (id: number) =>
  api.post(`/bots/${id}/stop`).then((r) => r.data)

export const deleteBot = (id: number) =>
  api.delete(`/bots/${id}`).then((r) => r.data)

// ─── Analytics ────────────────────────────────────────────────────────────────

export const getSummary = (days = 0, exchange = '', from?: string, to?: string) =>
  api.get<TradeSummary>('/analytics/summary', {
    params: {
      ...(days > 0 ? { days } : {}),
      ...(exchange ? { exchange } : {}),
      ...(from ? { from } : {}),
      ...(to ? { to } : {}),
    },
  }).then((r) => r.data)

export const getCoins = () =>
  api.get<CoinPerformance[]>('/analytics/coins').then((r) => r.data)

export const getSnapshots = (params: {
  days?: number
  hours?: number
  from?: string
  to?: string
} = {}) =>
  api.get<Snapshot[]>('/analytics/snapshots', { params }).then((r) => r.data)

export const getPortfolioChart = (
  range = '7d',
  exchange = 'all',
  from?: string,
  to?: string,
) =>
  api.get<ChartPoint[]>('/analytics/chart', {
    params: { range, exchange, ...(from && to ? { from, to } : {}) },
  }).then((r) => r.data)

export const takeSnapshot = () =>
  api.post('/analytics/snapshot').then((r) => r.data)

export const getArbitrage = (minSpread?: number) =>
  api.get<ArbitrageOpportunity[]>('/analytics/arbitrage', {
    params: minSpread !== undefined ? { min_spread: minSpread } : {},
  }).then((r) => r.data)

export const getTopSymbols = () =>
  api.get<string[]>('/market/top-symbols').then((r) => r.data)

// ─── DCA Bots ─────────────────────────────────────────────────────────────────

export const createDCA = (data: CreateDCAPayload) =>
  api.post<DCABot>('/dca', data).then((r) => r.data)

export const listDCA = () =>
  api.get<DCABot[]>('/dca').then((r) => r.data)

export const getDCA = (id: number) =>
  api.get<DCABot>(`/dca/${id}`).then((r) => r.data)

export const startDCA = (id: number, buyNow = false) =>
  api.post(`/dca/${id}/start`, null, { params: buyNow ? { buy_now: true } : {} }).then((r) => r.data)

export const stopDCA = (id: number) =>
  api.post(`/dca/${id}/stop`).then((r) => r.data)

export const deleteDCA = (id: number) =>
  api.delete(`/dca/${id}`).then((r) => r.data)

// ─── Portfolio extra ─────────────────────────────────────────────────────────

export const updateAssetPrice = (id: number, avg_buy_price: number, manually_set: boolean) =>
  api.patch(`/portfolio/assets/${id}/price`, { avg_buy_price, manually_set }).then((r) => r.data)

// ─── AI Advisor ──────────────────────────────────────────────────────────────

export interface AIMessage {
  role: 'user' | 'assistant'
  content: string
}

export const askAI = (message: string, history: AIMessage[]) =>
  api.post<{ reply: string }>('/ai/ask', { message, history }).then((r) => r.data)

// ─── Futures Positions ────────────────────────────────────────────────────────

export interface FuturesPosition {
  id: number
  user_id: number
  exchange: string
  symbol: string
  side: 'LONG' | 'SHORT'
  size: number
  entry_price: number
  mark_price: number
  unrealized_pnl: number
  leverage: number
  margin_type: string
  synced_at: string
}

export interface FuturesResponse {
  positions: FuturesPosition[]
  total_unrealized_pnl: number
}

export const getFuturesPositions = (exchange?: string) =>
  api.get<FuturesResponse>('/futures/positions', {
    params: exchange ? { exchange } : {},
  }).then((r) => r.data)

// ─── Spot Trades ──────────────────────────────────────────────────────────────

export interface SpotTrade {
  id: number
  user_id: number
  exchange: string
  symbol: string
  side: 'buy' | 'sell'
  quantity: number
  price: number
  fee: number
  fee_asset: string
  traded_at: number // Unix ms
}

export interface SpotTradesResponse {
  trades: SpotTrade[]
}

export const getSpotTrades = (exchange?: string) =>
  api.get<SpotTradesResponse>('/spot-trades', {
    params: exchange ? { exchange } : {},
  }).then((r) => r.data)

export default api
