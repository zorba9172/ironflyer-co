import { ViteReactSSG } from 'vite-react-ssg';
import '@ironflyer/ui-web/fonts.css';
import './styles.css';
import { routes } from './routes';

export const createRoot = ViteReactSSG({ routes });
