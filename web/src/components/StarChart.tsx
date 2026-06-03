import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import type { Snapshot } from '../api/types'

export function StarChart({ snapshots }: { snapshots: Snapshot[] }) {
  return (
    <div data-testid="star-chart" className="h-56 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={snapshots} margin={{ top: 8, right: 8, bottom: 8, left: 8 }}>
          <XAxis dataKey="date" tick={{ fontSize: 11 }} minTickGap={24} />
          <YAxis tick={{ fontSize: 11 }} width={48} />
          <Tooltip />
          <Line type="monotone" dataKey="stars" stroke="#2563eb" dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
