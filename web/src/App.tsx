import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Trending } from './pages/Trending'
import { RepositoryDetail } from './pages/RepositoryDetail'
import { Search } from './pages/Search'
import { Submit } from './pages/Submit'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Trending />} />
          <Route path="/trending/:period" element={<Trending />} />
          <Route path="/repositories/:id" element={<RepositoryDetail />} />
          <Route path="/search" element={<Search />} />
          <Route path="/submit" element={<Submit />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
