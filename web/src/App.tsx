import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Trending } from './pages/Trending'
import { RepositoryDetail } from './pages/RepositoryDetail'
import { Search } from './pages/Search'
import { Submit } from './pages/Submit'
import { Topics } from './pages/Topics'
import { TopicDetail } from './pages/TopicDetail'
import { Developers } from './pages/Developers'
import { Stats } from './pages/Stats'

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
          <Route path="/topics" element={<Topics />} />
          <Route path="/topics/:slug" element={<TopicDetail />} />
          <Route path="/trending/developers" element={<Developers />} />
          <Route path="/stats" element={<Stats />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
