import { ViteReactSSG } from 'vite-react-ssg';
import '@fontsource-variable/bricolage-grotesque';
import '@fontsource-variable/inter';
import '@fontsource/geist-mono/400.css';
import '@fontsource/geist-mono/500.css';
import './styles.css';
import { routes } from './routes';

export const createRoot = ViteReactSSG({ routes });
