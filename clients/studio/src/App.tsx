import { Box } from '@mui/material';
import { Routes, Route, useNavigate } from 'react-router-dom';
import { AppSidebar } from './components/AppSidebar';
import { StudioHome } from './pages/StudioHome';
import { Editor } from './pages/Editor';

function HomeLayout() {
  const navigate = useNavigate();
  return (
    <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default' }}>
      <AppSidebar onNewProject={() => navigate('/build')} />
      <StudioHome />
    </Box>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="/" element={<HomeLayout />} />
      <Route path="/build" element={<Editor />} />
    </Routes>
  );
}
