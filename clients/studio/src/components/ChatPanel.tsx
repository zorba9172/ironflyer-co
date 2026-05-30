import { useEffect, useMemo, useRef, useState, type DragEvent as ReactDragEvent } from 'react';
import {
  Avatar, Box, Chip, ClickAwayListener, IconButton, InputBase, MenuItem, Paper,
  Popover, Stack, TextField, ToggleButton, ToggleButtonGroup, Tooltip, Typography,
} from '@mui/material';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useChatStream, useAuth } from '@ironflyer/data';
import { formatRelativeTime, formatUSD } from '@ironflyer/core';
import {
  VscAdd, VscHistory, VscArchive, VscTrash, VscCopy, VscRefresh, VscDebugStop,
  VscEdit, VscChevronDown, VscChevronRight, VscRobot, VscLightbulb, VscShield,
  VscQuestion, VscChecklist, VscDebugStart, VscRocket, VscTerminal, VscPreview, VscSend,
  VscSparkle,
} from 'react-icons/vsc';
import { useStudio, buildFocusContext, type ChatMsg, type Attachment } from '../store';
import { AGENTS, type Agent } from '../studioData';
import { Markdown } from './Markdown';
import { PreflightDialog } from './PreflightDialog';
import { LogoMark } from './LogoMark';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { extractCodeFiles } from '../lib/extractCode';
import { text as fontScale } from '@ironflyer/design-tokens/brand';
import { studioTokens } from '../theme';

// ── Quick-start suggestions shown on the empty chat ───────────────────────────
const SUGGESTIONS = [
  'Scaffold a SaaS dashboard with auth',
  'Add Stripe checkout + webhook',
  'Design a landing page with a hero image',
  'Audit and close my security gates',
];

// ── Follow-ups offered after a reply lands ────────────────────────────────────
const FOLLOWUPS = [
  "What's still blocking shipping?",
  'Apply the proposed patches',
  'Run the security gates',
  'Explain what just changed',
];

// ── Types ─────────────────────────────────────────────────────────────────────
type WorkMode = 'ask' | 'plan' | 'execute' | 'autopilot';
type TrustChatMsg = ChatMsg & {
  mode?: WorkMode;
  filesExtracted?: string[];
  costUSD?: number;
  riskHints?: string[];
};

interface SlashCommand {
  cmd: string; desc: string; prompt?: string;
  action?: 'new' | 'clear' | 'retry' | 'review' | 'stop'; mode?: WorkMode;
}

const SLASH: SlashCommand[] = [
  { cmd: '/new', desc: 'Start a new chat', action: 'new' },
  { cmd: '/clear', desc: 'Clear this conversation', action: 'clear' },
  { cmd: '/ask', desc: 'Switch to Ask mode', mode: 'ask' },
  { cmd: '/plan', desc: 'Switch to Plan mode', mode: 'plan' },
  { cmd: '/execute', desc: 'Switch to Execute mode', mode: 'execute' },
  { cmd: '/autopilot', desc: 'Switch to Autopilot mode', mode: 'autopilot' },
  { cmd: '/retry', desc: 'Retry the last assistant reply', action: 'retry' },
  { cmd: '/review', desc: 'Review the last answer for risks', action: 'review' },
  { cmd: '/stop', desc: 'Stop the running reply', action: 'stop' },
  { cmd: '/explain', desc: 'Explain the current code & architecture', prompt: 'Explain the current codebase and architecture in plain terms — what each part does and how it fits together.' },
  { cmd: '/security', desc: 'Audit & close security gates', prompt: 'Audit my security gates, list every finding by severity, and propose reviewable patches to close them.' },
  { cmd: '/ship', desc: "What's blocking shipping?", prompt: "What is blocking shipping right now? List each open finisher gate and exactly what's needed to close it." },
  { cmd: '/optimize', desc: 'Find performance & cost wins', prompt: 'Review the build for performance and provider-cost wins. Name concrete changes and their expected impact.' },
];

const MODES: { id: WorkMode; label: string; tip: string; Icon: typeof VscQuestion }[] = [
  { id: 'ask', label: 'Ask', Icon: VscQuestion, tip: 'Answer and explain without taking action' },
  { id: 'plan', label: 'Plan', Icon: VscChecklist, tip: 'Pause on the pre-spend plan gate before dispatch' },
  { id: 'execute', label: 'Execute', Icon: VscDebugStart, tip: 'Implement a bounded change with reviewable evidence' },
  { id: 'autopilot', label: 'Autopilot', Icon: VscRocket, tip: 'Drive the work end-to-end after confirmation' },
];

// ── Pulsing "thinking" dots ────────────────────────────────────────────────────
function TypingDots() {
  return (
    <Stack direction="row" spacing={0.6} alignItems="center" sx={{ py: 0.75, '@keyframes ifPulse': { '0%,80%,100%': { opacity: 0.25, transform: 'scale(0.7)' }, '40%': { opacity: 1, transform: 'scale(1)' } } }}>
      {[0, 1, 2].map((i) => (
        <Box key={i} sx={(t) => ({ width: 6, height: 6, borderRadius: 99, bgcolor: t.palette.primary.main, opacity: 0.7, animation: 'ifPulse 1.2s ease-in-out infinite', animationDelay: `${i * 0.2}s` })} />
      ))}
    </Stack>
  );
}

// ── Language detection / i18n ─────────────────────────────────────────────────
type Lang = string;
const LATIN_HINTS: [Lang, RegExp][] = [
  ['es', /\b(el|los|las|una|por favor|gracias|quiero|necesito|construir|aplicaci[oó]n|noticias|hola)\b/i],
  ['pt', /\b(uma|por favor|obrigad|quero|preciso|construir|aplicativo|not[ií]cias|ol[aá])\b/i],
  ['fr', /\b(le|les|une|s'il vous pla[iî]t|merci|je veux|construire|application|actualit[eé]s|bonjour)\b/i],
  ['de', /\b(der|die|das|eine|bitte|danke|ich m[oö]chte|bauen|anwendung|nachrichten|hallo)\b/i],
  ['it', /\b(gli|una|per favore|grazie|voglio|costruire|applicazione|notizie|ciao)\b/i],
];

function detectLang(text: string): Lang {
  const s = (text || '').trim();
  if (!s) return 'en';
  if (/[֐-׿]/.test(s)) return 'he';
  if (/[؀-ۿ]/.test(s)) return 'ar';
  if (/[Ѐ-ӿ]/.test(s)) return 'ru';
  if (/[぀-ヿ]/.test(s)) return 'ja';
  if (/[가-힯]/.test(s)) return 'ko';
  if (/[ऀ-ॿ]/.test(s)) return 'hi';
  if (/[一-鿿]/.test(s)) return 'zh';
  for (const [code, re] of LATIN_HINTS) if (re.test(s)) return code;
  return 'en';
}

type MsgKey =
  | 'budgetTooLow' | 'insufficientFunds' | 'budgetMid' | 'unauth' | 'unavailable'
  | 'askBudgetBlocked'
  | 'cancelled' | 'offline' | 'rejected' | 'offlineReply' | 'dropTitle' | 'dropSub'
  | 'ctxFull' | 'newChat';

const STATUS: Record<MsgKey, Record<Lang, string>> = {
  budgetTooLow: {
    en: 'The budget set for this build is too low for what you asked. To build it end to end, [upgrade your plan](/plans) or raise the budget, then run again.',
    he: 'ה-budget שהוקצב נמוך מדי למה שביקשת לבנות. כדי שאבנה את זה מקצה-לקצה — [שדרג את ה-plan](/plans) או הגדל את ה-budget, ואריץ שוב.',
    es: 'El budget asignado es demasiado bajo para lo que pediste. Para construirlo de principio a fin, [mejora tu plan](/plans) o aumenta el budget, y vuelve a ejecutar.',
    pt: 'O budget definido é baixo demais para o que você pediu. Para construir de ponta a ponta, [atualize seu plan](/plans) ou aumente o budget e execute de novo.',
    fr: 'Le budget défini est trop bas pour votre demande. Pour le construire de bout en bout, [améliorez votre plan](/plans) ou augmentez le budget, puis relancez.',
    de: 'Das budget ist zu niedrig für deine Anfrage. Um es vollständig zu bauen, [aktualisiere deinen plan](/plans) oder erhöhe das budget und starte erneut.',
    it: 'Il budget impostato è troppo basso per ciò che hai chiesto. Per costruirlo da cima a fondo, [aggiorna il plan](/plans) o aumenta il budget, poi riprova.',
    ar: 'الـ budget المحدد منخفض جدًا لما طلبته. لبنائه بالكامل، [قم بترقية الـ plan](/plans) أو زِد الـ budget ثم أعد التشغيل.',
    ru: 'Заданный budget слишком мал для запрошенного. Чтобы собрать всё целиком, [обновите plan](/plans) или увеличьте budget и запустите снова.',
  },
  insufficientFunds: {
    en: 'Your wallet is empty. [Add credits](/plans) so I can start building.',
    he: 'ה-wallet שלך ריק. [טען credits](/plans) כדי שאתחיל לבנות.',
    es: 'Tu wallet está vacío. [Agrega credits](/plans) para que pueda empezar a construir.',
    pt: 'Seu wallet está vazio. [Adicione credits](/plans) para eu começar a construir.',
    fr: 'Votre wallet est vide. [Ajoutez des credits](/plans) pour que je commence à construire.',
    de: 'Dein wallet ist leer. [Füge credits hinzu](/plans), damit ich anfangen kann.',
    it: 'Il tuo wallet è vuoto. [Aggiungi credits](/plans) così posso iniziare a costruire.',
    ar: 'الـ wallet فارغ. [أضف credits](/plans) لأبدأ البناء.',
    ru: 'Ваш wallet пуст. [Добавьте credits](/plans), чтобы я начал сборку.',
  },
  budgetMid: {
    en: 'This build ran out of budget partway through. [Top up your wallet](/plans) to continue.',
    he: 'ה-budget של ההרצה אזל באמצע הבנייה. [טען את ה-wallet](/plans) כדי להמשיך.',
    es: 'El build se quedó sin budget a mitad de camino. [Recarga tu wallet](/plans) para continuar.',
    pt: 'O build ficou sem budget no meio. [Recarregue seu wallet](/plans) para continuar.',
    fr: 'Le build a épuisé son budget en cours de route. [Rechargez votre wallet](/plans) pour continuer.',
    de: 'Dem build ging mittendrin das budget aus. [Lade dein wallet auf](/plans), um fortzufahren.',
    it: 'Il build ha esaurito il budget a metà. [Ricarica il wallet](/plans) per continuare.',
    ar: 'نفد الـ budget في منتصف البناء. [اشحن الـ wallet](/plans) للمتابعة.',
    ru: 'У сборки закончился budget на полпути. [Пополните wallet](/plans), чтобы продолжить.',
  },
  askBudgetBlocked: {
    en: 'Ask mode does not change files, but a live orchestrator reply still needs wallet credit. Add credits or keep working locally in the editor.',
    he: 'Ask mode לא משנה קבצים, אבל תשובת Orchestrator חיה עדיין דורשת credit ב-wallet. טען credits או המשך לעבוד מקומית באדיטור.',
    es: 'Ask mode no cambia archivos, pero una respuesta live del orchestrator aún necesita credit en el wallet. Agrega credits o sigue trabajando localmente en el editor.',
    pt: 'Ask mode não altera arquivos, mas uma resposta live do orchestrator ainda precisa de credit no wallet. Adicione credits ou continue localmente no editor.',
    fr: 'Ask mode ne modifie pas les fichiers, mais une réponse live de l’orchestrator nécessite encore du credit dans le wallet. Ajoutez des credits ou continuez localement dans l’éditeur.',
    de: 'Ask mode ändert keine Dateien, aber eine live Antwort des orchestrator braucht weiterhin credit im wallet. Füge credits hinzu oder arbeite lokal im Editor weiter.',
    it: 'Ask mode non modifica file, ma una risposta live dell’orchestrator richiede comunque credit nel wallet. Aggiungi credits o continua localmente nell’editor.',
    ar: 'Ask mode لا يغيّر الملفات، لكن رد orchestrator المباشر ما زال يحتاج credit في الـ wallet. أضف credits أو تابع العمل محليًا في المحرر.',
    ru: 'Ask mode не меняет файлы, но live-ответ orchestrator всё равно требует credit в wallet. Добавьте credits или продолжайте локально в редакторе.',
  },
  unauth: {
    en: 'Your session expired. Please sign in again to continue.',
    he: 'ה-session פג. התחבר מחדש כדי להמשיך.',
    es: 'Tu session expiró. Inicia sesión de nuevo para continuar.',
    pt: 'Sua session expirou. Faça login novamente para continuar.',
    fr: 'Votre session a expiré. Reconnectez-vous pour continuer.',
    de: 'Deine session ist abgelaufen. Bitte melde dich erneut an.',
    it: 'La tua session è scaduta. Accedi di nuovo per continuare.',
    ar: 'انتهت الـ session. سجّل الدخول من جديد للمتابعة.',
    ru: 'Ваша session истекла. Войдите снова, чтобы продолжить.',
  },
  unavailable: {
    en: 'The assistant is temporarily unavailable. Please try again in a moment.',
    he: 'העוזר אינו זמין כרגע. נסה שוב בעוד רגע.',
    es: 'El asistente no está disponible por ahora. Inténtalo de nuevo en un momento.',
    pt: 'O assistente está indisponível no momento. Tente novamente em instantes.',
    fr: "L'assistant est momentanément indisponible. Réessayez dans un instant.",
    de: 'Der Assistent ist vorübergehend nicht verfügbar. Bitte versuche es gleich erneut.',
    it: "L'assistente non è disponibile al momento. Riprova tra poco.",
    ar: 'المساعد غير متاح مؤقتًا. حاول مرة أخرى بعد قليل.',
    ru: 'Ассистент временно недоступен. Повторите попытку через мгновение.',
  },
  cancelled: {
    en: 'The request was cancelled.', he: 'הבקשה בוטלה.', es: 'La solicitud fue cancelada.',
    pt: 'A solicitação foi cancelada.', fr: 'La requête a été annulée.', de: 'Die Anfrage wurde abgebrochen.',
    it: 'La richiesta è stata annullata.', ar: 'تم إلغاء الطلب.', ru: 'Запрос отменён.',
  },
  offline: {
    en: "You're in preview mode — connect the studio to go live.",
    he: 'אתה ב-preview mode — חבר את ה-studio כדי לעבוד חי.',
    es: 'Estás en preview mode — conecta el studio para ir en vivo.',
    pt: 'Você está em preview mode — conecte o studio para ir ao vivo.',
    fr: 'Vous êtes en preview mode — connectez le studio pour passer en direct.',
    de: 'Du bist im preview mode — verbinde das studio, um live zu gehen.',
    it: 'Sei in preview mode — collega lo studio per andare in diretta.',
    ar: 'أنت في preview mode — اربط الـ studio للعمل المباشر.',
    ru: 'Вы в preview mode — подключите studio для работы вживую.',
  },
  rejected: {
    en: 'That request was rejected. Please retry, or refresh the studio if it persists.',
    he: 'הבקשה נדחתה. נסה שוב, או רענן את ה-studio אם זה חוזר.',
    es: 'Esa solicitud fue rechazada. Reinténtalo o recarga el studio si persiste.',
    pt: 'Essa solicitação foi rejeitada. Tente de novo ou recarregue o studio se persistir.',
    fr: 'Cette requête a été rejetée. Réessayez, ou rafraîchissez le studio si cela persiste.',
    de: 'Diese Anfrage wurde abgelehnt. Versuche es erneut oder lade das studio neu.',
    it: 'Richiesta rifiutata. Riprova o ricarica lo studio se persiste.',
    ar: 'تم رفض الطلب. أعد المحاولة أو حدّث الـ studio إذا استمر.',
    ru: 'Запрос отклонён. Повторите или обновите studio, если повторится.',
  },
  offlineReply: {
    en: '"{q}" is queued as a finisher request. The studio is in preview mode, so live runs are paused; the review, security, and economics surfaces are showing sample project state.',
    he: '"{q}" נכנס כבקשת finisher. ה-studio ב-preview mode, אז live runs מושהים; משטחי ה-review, security וה-economics מציגים sample data של הפרויקט.',
    es: '"{q}" quedó en cola como solicitud del finisher. El studio está en preview mode, así que los live runs están en pausa; las vistas de review, security y economics muestran datos de ejemplo.',
    pt: '"{q}" entrou na fila como pedido do finisher. O studio está em preview mode, então live runs estão pausados; as telas de review, security e economics mostram dados de exemplo.',
    fr: "\"{q}\" est mis en file comme requête finisher. Le studio est en preview mode, donc les live runs sont en pause ; les vues review, security et economics affichent des données d'exemple.",
    de: '"{q}" ist als finisher-Anfrage eingereiht. Das studio ist im preview mode, daher pausieren live runs; die review-, security- und economics-Ansichten zeigen Beispieldaten.',
    it: '"{q}" è in coda come richiesta del finisher. Lo studio è in preview mode, quindi i live runs sono in pausa; le viste review, security ed economics mostrano dati di esempio.',
    ar: '"{q}" في قائمة الانتظار كطلب finisher. الـ studio في preview mode، لذا توقفت الـ live runs مؤقتًا؛ وتعرض شاشات review وsecurity وeconomics بيانات تجريبية.',
    ru: '«{q}» поставлен в очередь как запрос finisher. Studio в preview mode, поэтому live runs приостановлены; экраны review, security и economics показывают демоданные.',
  },
  dropTitle: {
    en: 'Drop to attach', he: 'שחרר כדי לצרף', es: 'Suelta para adjuntar', pt: 'Solte para anexar',
    fr: 'Déposez pour joindre', de: 'Zum Anhängen ablegen', it: 'Rilascia per allegare',
    ar: 'أفلت للإرفاق', ru: 'Отпустите, чтобы прикрепить',
  },
  dropSub: {
    en: 'docs · images · zip · anything — used as build context',
    he: 'מסמכים · תמונות · zip · הכל — ישמש כ-context לבנייה',
    es: 'docs · imágenes · zip · cualquier cosa — se usa como context de build',
    pt: 'docs · imagens · zip · qualquer coisa — usado como context do build',
    fr: 'docs · images · zip · tout — utilisé comme context de build',
    de: 'docs · Bilder · zip · alles — als build-context verwendet',
    it: 'docs · immagini · zip · qualsiasi cosa — usato come context di build',
    ar: 'مستندات · صور · zip · أي شيء — يُستخدم كـ context للبناء',
    ru: 'docs · изображения · zip · что угодно — используется как build-context',
  },
  ctxFull: {
    en: 'This chat is long and the context is filling up — open a new chat for sharper results.',
    he: 'השיחה ארוכה וה-context מתחיל להתמלא — פתח chat חדש לתוצאות מדויקות.',
    es: 'Este chat es largo y el context se está llenando — abre un chat nuevo para mejores resultados.',
    pt: 'Este chat está longo e o context está enchendo — abra um chat novo para resultados melhores.',
    fr: 'Ce chat est long et le context se remplit — ouvrez un nouveau chat pour de meilleurs résultats.',
    de: 'Dieser chat ist lang und der context füllt sich — öffne einen neuen chat für bessere Ergebnisse.',
    it: 'Questo chat è lungo e il context si sta riempiendo — apri un nuovo chat per risultati migliori.',
    ar: 'هذه الـ chat طويلة والـ context يمتلئ — افتح chat جديدة لنتائج أدق.',
    ru: 'Этот chat длинный, и context заполняется — откройте новый chat для точных результатов.',
  },
  newChat: {
    en: 'New chat', he: 'chat חדש', es: 'Nuevo chat', pt: 'Novo chat', fr: 'Nouveau chat',
    de: 'Neuer chat', it: 'Nuovo chat', ar: 'chat جديدة', ru: 'Новый chat',
  },
};

function t(key: MsgKey, lang: Lang): string {
  const row = STATUS[key];
  return row[lang] ?? row.en ?? '';
}

function offlineReply(prompt: string): string {
  const q = prompt.length > 80 ? `${prompt.slice(0, 80)}…` : prompt;
  return t('offlineReply', detectLang(prompt)).replace('{q}', q);
}

// ── Vendor scrub — never leak provider names to the UI ────────────────────────
const VENDOR_RE = /\b(gemini|google\s*ai|vertex|anthropic|claude|openai|gpt-?\d|deepseek|hugging\s?face|llama|qwen|mistral|mixtral|bedrock|azure)\b/i;

function chatStatusMessage(code: string, raw: string, lang: Lang, mode?: WorkMode): string {
  const budgetBlocked = mode === 'ask' ? t('askBudgetBlocked', lang) : null;
  switch (code) {
    case 'BUDGET_TOO_LOW':
    case 'PROFITGUARD_REFUSED':
    case 'PROFITGUARD':
      return budgetBlocked ?? t('budgetTooLow', lang);
    case 'INSUFFICIENT_FUNDS':
      return budgetBlocked ?? t('insufficientFunds', lang);
    case 'BUDGET':
      return budgetBlocked ?? t('budgetMid', lang);
    case 'UNAUTHENTICATED':
      return t('unauth', lang);
    case 'NO_PROVIDER':
    case 'UNAVAILABLE':
      return t('unavailable', lang);
    case 'CANCELLED':
      return t('cancelled', lang);
  }
  if (/insufficient wallet|budget_exhausted|payment required|402|out of (credit|budget)/i.test(raw)) return budgetBlocked ?? t('budgetMid', lang);
  if (/unauth|session expired|\b401\b/i.test(raw)) return t('unauth', lang);
  if (/offline|no orchestrator endpoint/i.test(raw)) return t('offline', lang);
  if (/chat stream failed:\s*4\d\d/i.test(raw)) return t('rejected', lang);
  if (/chat stream failed:\s*5\d\d/i.test(raw) || !raw.trim() || VENDOR_RE.test(raw) || raw.trim().startsWith('{') || raw.length > 200) return t('unavailable', lang);
  return raw;
}

function extractErrorCode(error: unknown): string {
  if (error && typeof error === 'object' && 'code' in error) {
    const c = (error as { code?: unknown }).code;
    if (typeof c === 'string') return c;
  }
  return '';
}

function resolveChatError(input: { code?: string; message?: string; error?: unknown }, lang: Lang, mode?: WorkMode): string {
  const code = input.code || extractErrorCode(input.error);
  const raw = input.message ?? (input.error instanceof Error ? input.error.message : String(input.error ?? ''));
  return chatStatusMessage(code, raw, lang, mode);
}

// ── Utilities ─────────────────────────────────────────────────────────────────
const uid = (p: string) => `${p}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
const slug = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '');
const safeLabel = (value: string, fallback: string) => (VENDOR_RE.test(value) ? fallback : value);
function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.map((v) => v.trim()).filter(Boolean))];
}

// ── File drop ingest ──────────────────────────────────────────────────────────
const DROP_TEXT_RE = /\.(txt|md|markdown|json|ya?ml|csv|tsv|html?|xml|js|jsx|ts|tsx|css|scss|py|go|rs|java|rb|php|sh|sql|toml|ini|env|dockerfile|log)$/i;
const DROP_MAX_TEXT_BYTES = 256 * 1024;

function readFileAs(file: File, how: 'text' | 'dataURL'): Promise<string> {
  return new Promise((resolve) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result ?? ''));
    r.onerror = () => resolve('');
    if (how === 'text') r.readAsText(file);
    else r.readAsDataURL(file);
  });
}

async function readDroppedFiles(list: FileList): Promise<Attachment[]> {
  const out: Attachment[] = [];
  for (const file of Array.from(list)) {
    const base = { id: uid('att_'), name: file.name, size: file.size };
    if (file.type.startsWith('image/')) {
      out.push({ ...base, kind: 'image', dataUrl: await readFileAs(file, 'dataURL') });
    } else if ((file.type.startsWith('text/') || DROP_TEXT_RE.test(file.name)) && file.size <= DROP_MAX_TEXT_BYTES) {
      out.push({ ...base, kind: 'text', text: await readFileAs(file, 'text') });
    } else {
      out.push({ ...base, kind: 'file' });
    }
  }
  return out;
}

function modeInstruction(mode: WorkMode): string {
  switch (mode) {
    case 'ask': return 'Mode: Ask. Answer directly, explain tradeoffs, and do not claim to have changed files unless files are actually emitted.';
    case 'plan': return 'Mode: Plan. Produce a concise implementation plan, risks, files likely to change, and acceptance checks before any execution.';
    case 'execute': return 'Mode: Execute. Implement the requested bounded change, surface reviewable patches/evidence, and call out remaining checks.';
    case 'autopilot': return 'Mode: Autopilot. Drive the request end-to-end, route work to specialists when useful, and report evidence, cost, and risk at each gate.';
  }
}

function riskHintsFor(prompt: string, mode: WorkMode, planMode: boolean): string[] {
  const hints: string[] = [];
  if (planMode) hints.push('preflight gate');
  if (mode === 'ask') hints.push('no file changes');
  if (mode === 'execute' || mode === 'autopilot') hints.push('review patches before apply');
  if (mode === 'autopilot') hints.push('higher autonomy');
  if (/(secret|token|key|auth|payment|stripe|billing|wallet|database|migration|delete|prod|security)/i.test(prompt)) hints.push('sensitive surface');
  return uniqueStrings(hints);
}

function reviewPromptFor(msg: TrustChatMsg): string {
  const quote = msg.text.length > 1600 ? `${msg.text.slice(0, 1600)}\n... [trimmed]` : msg.text;
  return `Review your previous answer for correctness, hidden risk, missing tests, and unsafe assumptions. Keep it provider-blind and action-oriented.\n\nPrevious answer:\n${quote}`;
}

interface Mentionable { token: string; name: string; role: string }
function mentionables(custom: Agent[]): Mentionable[] {
  return [
    ...AGENTS.map((a) => ({ token: a.id, name: a.name, role: a.role })),
    ...custom.map((a) => ({ token: slug(a.name || a.id), name: a.name || 'Untitled agent', role: a.role || 'custom agent' })),
  ];
}

function resolveMentions(text: string, all: Mentionable[]): Mentionable[] {
  const tokens = new Set((text.match(/@([a-z0-9_]+)/gi) ?? []).map((m) => m.slice(1).toLowerCase()));
  return all.filter((m) => tokens.has(m.token.toLowerCase()));
}

// ── Main component ─────────────────────────────────────────────────────────────
export function ChatPanel({ initialPrompt }: { initialPrompt?: string }) {
  const mockProjectId = useStudio((s) => s.current.id);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const launchWorkMode = useStudio((s) => s.initialWorkMode);
  const launchPreflight = useStudio((s) => s.initialPreflight);
  const constitution = useStudio((s) => s.constitution);
  const attachments = useStudio((s) => s.attachments);
  const addAttachments = useStudio((s) => s.addAttachments);
  const writeGeneratedFiles = useStudio((s) => s.writeGeneratedFiles);
  const customAgents = useStudio((s) => s.customAgents);
  const chatSessions = useStudio((s) => s.chatSessions);
  const activeChatId = useStudio((s) => s.activeChatId);
  const ensureChat = useStudio((s) => s.ensureChat);
  const newChat = useStudio((s) => s.newChat);
  const selectChat = useStudio((s) => s.selectChat);
  const commitChat = useStudio((s) => s.commitChat);
  const renameChat = useStudio((s) => s.renameChat);
  const archiveChat = useStudio((s) => s.archiveChat);
  const restoreChat = useStudio((s) => s.restoreChat);
  const deleteChat = useStudio((s) => s.deleteChat);
  const repairRequest = useStudio((s) => s.repairRequest);
  const clearRepairRequest = useStudio((s) => s.clearRepairRequest);

  const { isLive, send: streamSend } = useChatStream();
  const { signOut } = useAuth();
  const liveProjectId = useLiveProjectId();

  const contextSentRef = useRef(false);
  const startedRef = useRef(false);
  const loadedIdRef = useRef<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const [messages, setMessages] = useState<TrustChatMsg[]>([]);
  const [draft, setDraft] = useState('');
  const [thinking, setThinking] = useState(false);
  const [dragOver, setDragOver] = useState(false);

  const uiLang: Lang = useMemo(() => {
    const lastUser = [...messages].reverse().find((m) => m.from === 'user')?.text ?? '';
    return detectLang(draft || lastUser);
  }, [messages, draft]);

  const ctxChars = useMemo(() => messages.reduce((n, m) => n + m.text.length, 0), [messages]);
  const ctxNearFull = messages.length >= 24 || ctxChars > 24000;

  const onDropFiles = async (e: ReactDragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const files = e.dataTransfer?.files;
    if (!files || files.length === 0) return;
    const items = await readDroppedFiles(files);
    if (items.length > 0) addAttachments(items);
  };

  const [historyAnchor, setHistoryAnchor] = useState<HTMLElement | null>(null);
  const [slashOpen, setSlashOpen] = useState(false);
  const [workMode, setWorkMode] = useState<WorkMode>(launchWorkMode);
  const [planMode, setPlanMode] = useState(launchPreflight);
  const [pending, setPending] = useState<{ text: string; history: TrustChatMsg[]; routed: Mentionable[]; mode: WorkMode } | null>(null);

  const activeSession = useMemo(() => chatSessions.find((c) => c.id === activeChatId) ?? null, [chatSessions, activeChatId]);
  const targetProjectId = storeProjectId ?? liveProjectId ?? mockProjectId;

  useEffect(() => { ensureChat(); }, [ensureChat]);

  useEffect(() => {
    if (!activeChatId || activeChatId === loadedIdRef.current) return;
    abortRef.current?.abort();
    loadedIdRef.current = activeChatId;
    contextSentRef.current = false;
    setThinking(false);
    setMessages(chatSessions.find((c) => c.id === activeChatId)?.messages ?? []);
  }, [activeChatId, chatSessions]);

  const commit = (msgs: TrustChatMsg[]) => { if (loadedIdRef.current) commitChat(loadedIdRef.current, msgs); };

  const respond = async (prompt: string, history: TrustChatMsg[], routed: Mentionable[] = [], mode: WorkMode = workMode) => {
    setThinking(true);
    if (isLive) {
      const id = uid('a');
      setMessages((m) => [...m, { id, from: 'agent', text: '', steps: [], ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined, riskHints: riskHintsFor(prompt, mode, mode === 'plan' || mode === 'autopilot') }]);
      const patch = (fn: (x: TrustChatMsg) => TrustChatMsg) => setMessages((m) => m.map((x) => (x.id === id ? fn(x) : x)));
      const parts: string[] = [];
      if (!contextSentRef.current) {
        const ctx = buildFocusContext(constitution, attachments);
        if (ctx) { parts.push(ctx); contextSentRef.current = true; }
      }
      if (routed.length > 0) {
        parts.push(`# Route this request\nHand this to the following specialist${routed.length === 1 ? '' : 's'} and answer in that role:\n${routed.map((r) => `- ${r.name} — ${r.role}`).join('\n')}`);
      }
      const prior = history.slice(-8);
      if (prior.length > 0) {
        const trim = (s: string) => (s.length > 2000 ? `${s.slice(0, 2000)}\n… [trimmed] …` : s);
        const transcript = prior.map((x) => `[${x.from === 'user' ? 'User' : 'Ironflyer'}] ${trim(x.text)}`).join('\n\n');
        parts.push(`# Conversation so far\n${transcript}`);
      }
      parts.push(`# Mode\n${modeInstruction(mode)}`);
      parts.push(`# Request\n${prompt}`);
      const serverPrompt = parts.join('\n\n---\n\n');
      const lang = detectLang(prompt);
      const controller = new AbortController();
      abortRef.current = controller;
      let acc = '';
      try {
        await streamSend(targetProjectId, serverPrompt, (ev) => {
          if (ev.type === 'text') {
            acc += ev.text;
            patch((x) => ({ ...x, text: x.text + ev.text }));
            const files = extractCodeFiles(acc);
            if (files.length > 0) {
              writeGeneratedFiles(files);
              patch((x) => ({ ...x, filesExtracted: uniqueStrings([...(x.filesExtracted ?? []), ...files.map((f) => f.path)]) }));
            }
          } else if (ev.type === 'thinking') patch((x) => ({ ...x, thinking: (x.thinking ?? '') + ev.text }));
          else if (ev.type === 'tool') patch((x) => ({ ...x, steps: [...(x.steps ?? []), safeLabel(ev.name, 'tool step')] }));
          else if (ev.type === 'finish') patch((x) => ({ ...x, costUSD: typeof ev.costUSD === 'number' ? ev.costUSD : x.costUSD }));
          else if (ev.type === 'error') patch((x) => ({ ...x, text: x.text || `⚠ ${resolveChatError({ code: ev.code, message: ev.message }, lang, mode)}` }));
        }, controller.signal);
      } catch (e) {
        const raw = e instanceof Error ? e.message : String(e || '');
        if (controller.signal.aborted || /abort/i.test(raw)) {
          patch((x) => ({ ...x, text: x.text ? `${x.text}\n\n_⏹ stopped_` : '_⏹ stopped before any reply_' }));
        } else {
          patch((x) => ({ ...x, text: x.text || `⚠ ${resolveChatError({ error: e }, lang, mode)}` }));
          if (/unauth/i.test(raw)) void signOut();
        }
      } finally {
        abortRef.current = null;
        setThinking(false);
        setMessages((m) => { commit(m); return m; });
      }
    } else {
      window.setTimeout(() => {
        setMessages((m) => {
          const next: TrustChatMsg[] = [...m, { id: uid('a'), from: 'agent' as const, text: offlineReply(prompt), ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined, riskHints: riskHintsFor(prompt, mode, mode === 'plan' || mode === 'autopilot') }];
          commit(next);
          return next;
        });
        setThinking(false);
      }, 600);
    }
  };

  useEffect(() => {
    if (!initialPrompt || startedRef.current) return;
    if (messages.length > 0 || (activeSession && activeSession.messages.length > 0)) { startedRef.current = true; return; }
    if (isLive && liveProjectId === null) return;
    startedRef.current = true;
    const seed: TrustChatMsg = { id: uid('u'), from: 'user', text: initialPrompt, ts: Date.now(), mode: workMode };
    setMessages([seed]);
    commit([seed]);
    void respond(initialPrompt, [], [], workMode);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialPrompt, isLive, liveProjectId, activeSession]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, thinking]);

  const allMentions = useMemo(() => mentionables(customAgents), [customAgents]);

  const dispatchTurn = (text: string, history: TrustChatMsg[], routed: Mentionable[], mode: WorkMode) => {
    const user: TrustChatMsg = { id: uid('u'), from: 'user', text, ts: Date.now(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined };
    const next = [...history, user];
    setMessages(next);
    commit(next);
    void respond(text, history, routed, mode);
  };

  const send = (override?: string) => {
    const text = (override ?? draft).trim();
    if (!text || thinking) return;
    const history = messages;
    const routed = resolveMentions(text, allMentions);
    const mode = workMode;
    setDraft('');
    setSlashOpen(false);
    if (planMode && isLive) { setPending({ text, history, routed, mode }); return; }
    dispatchTurn(text, history, routed, mode);
  };

  const confirmPending = () => {
    if (!pending) return;
    const { text, history, routed, mode } = pending;
    setPending(null);
    dispatchTurn(text, history, routed, mode);
  };

  const editUser = (id: string, newText: string) => {
    const idx = messages.findIndex((m) => m.id === id);
    if (idx < 0 || !newText.trim()) return;
    const history = messages.slice(0, idx);
    const routed = resolveMentions(newText, allMentions);
    const mode = workMode;
    const edited: TrustChatMsg = { ...messages[idx]!, text: newText.trim(), mode, routedTo: routed.length ? routed.map((r) => r.name) : undefined };
    const trimmed = [...history, edited];
    setMessages(trimmed);
    commit(trimmed);
    void respond(newText.trim(), history, routed, mode);
  };

  const stop = () => { abortRef.current?.abort(); };

  useEffect(() => {
    if (!repairRequest || thinking) return;
    clearRepairRequest();
    send(repairRequest);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repairRequest, thinking]);

  const handleNewChat = () => {
    abortRef.current?.abort();
    startedRef.current = true;
    newChat();
  };

  const handleClear = () => {
    abortRef.current?.abort();
    setThinking(false);
    setMessages([]);
    commit([]);
  };

  const chooseMode = (mode: WorkMode) => {
    setWorkMode(mode);
    setPlanMode(mode === 'plan' || mode === 'autopilot');
  };

  const retryLast = () => {
    if (thinking) return;
    let lastUser = -1;
    for (let i = messages.length - 1; i >= 0; i--) if (messages[i]!.from === 'user') { lastUser = i; break; }
    if (lastUser < 0) return;
    const user = messages[lastUser]!;
    const text = user.text;
    const history = messages.slice(0, lastUser);
    const trimmed = messages.slice(0, lastUser + 1);
    setMessages(trimmed);
    commit(trimmed);
    void respond(text, history, resolveMentions(text, allMentions), user.mode ?? workMode);
  };

  const reviewLast = () => {
    if (thinking) return;
    const last = [...messages].reverse().find((m) => m.from === 'agent' && m.text);
    if (last) send(reviewPromptFor(last));
  };

  const runSlash = (c: SlashCommand) => {
    setDraft('');
    setSlashOpen(false);
    if (c.mode) return chooseMode(c.mode);
    if (c.action === 'new') return handleNewChat();
    if (c.action === 'clear') return handleClear();
    if (c.action === 'retry') return retryLast();
    if (c.action === 'review') return reviewLast();
    if (c.action === 'stop') return stop();
    if (c.prompt) send(c.prompt);
  };

  const slashMatches = draft.startsWith('/') && !draft.includes(' ')
    ? SLASH.filter((c) => c.cmd.startsWith(draft.toLowerCase()))
    : [];
  const slashCommands = slashOpen ? SLASH : slashMatches;

  const mentionQuery = draft.match(/@([a-z0-9_]*)$/i);
  const mentionMatches = !slashOpen && mentionQuery
    ? allMentions.filter((m) => m.token.toLowerCase().includes(mentionQuery[1]!.toLowerCase()) || slug(m.name).includes(mentionQuery[1]!.toLowerCase())).slice(0, 6)
    : [];
  const pickMention = (m: Mentionable) => setDraft((d) => `${d.replace(/@([a-z0-9_]*)$/i, '')}@${m.token} `);

  const ctxChips = useMemo(() => {
    const chips: { label: string; title: string }[] = [];
    if (constitution.trim()) chips.push({ label: 'constitution', title: 'The project rules the finisher must honor' });
    if (attachments.length) chips.push({ label: `${attachments.length} reference${attachments.length === 1 ? '' : 's'}`, title: attachments.map((a) => a.name).join(', ') });
    return chips;
  }, [constitution, attachments]);

  const lastAgentId = useMemo(() => [...messages].reverse().find((m) => m.from === 'agent' && m.text)?.id, [messages]);
  const showFollowups = !thinking && messages.length > 0 && messages[messages.length - 1]?.from === 'agent' && !!messages[messages.length - 1]?.text;

  // Virtualize transcript — only visible rows are in DOM
  const rowVirtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 110,
    overscan: 8,
    getItemKey: (i) => messages[i]!.id,
  });

  return (
    <Box
      onDragOver={(e) => { e.preventDefault(); if (!dragOver) setDragOver(true); }}
      onDragLeave={(e) => { if (e.currentTarget === e.target) setDragOver(false); }}
      onDrop={(e) => { void onDropFiles(e); }}
      sx={(theme) => ({
        position: 'relative',
        width: { xs: 380, xl: 488 },
        flexShrink: 0,
        height: '100%',
        borderRight: `1px solid ${theme.palette.divider}`,
        display: { xs: 'none', md: 'flex' },
        flexDirection: 'column',
        bgcolor: 'background.paper',
        transition: `background-color ${theme.studio.motion.base}`,
      })}
    >
      {/* Drop overlay */}
      {dragOver && (
        <Box sx={(t) => ({
          position: 'absolute', inset: 8, zIndex: 30, display: 'grid', placeItems: 'center',
          textAlign: 'center', p: 2, borderRadius: `${t.studio.radius.lg}px`,
          border: `2px dashed ${t.palette.primary.main}`,
          bgcolor: `${t.palette.background.default}f2`,
          backdropFilter: 'blur(8px)', pointerEvents: 'none',
          boxShadow: '0 12px 30px rgba(24,22,20,0.08)',
        })}>
          <Box>
            <VscSparkle size={24} style={{ color: studioTokens.neon.violet, marginBottom: 8 }} />
            <Typography sx={{ fontSize: fontScale.s95, fontWeight: 700, color: 'primary.main', mb: 0.5 }}>
              {t('dropTitle', uiLang)}
            </Typography>
            <Typography sx={{ fontSize: fontScale.s78, color: 'text.secondary' }}>
              {t('dropSub', uiLang)}
            </Typography>
          </Box>
        </Box>
      )}

      {/* Header */}
      <ChatHeader
        title={activeSession?.title ?? 'New chat'}
        onNew={handleNewChat}
        onOpenHistory={(el) => setHistoryAnchor(el)}
        onRename={(ttl) => activeChatId && renameChat(activeChatId, ttl)}
      />

      {/* Transcript */}
      <Box ref={scrollRef} sx={{ flex: 1, overflowY: 'auto', px: 2, pt: 2, pb: 0.5 }}>
        {messages.length === 0 && !thinking && (
          <EmptyState onSuggest={send} />
        )}

        {messages.length > 0 && (
          <Box sx={{ position: 'relative', width: '100%', height: rowVirtualizer.getTotalSize() }}>
            {rowVirtualizer.getVirtualItems().map((vi) => {
              const m = messages[vi.index]!;
              return (
                <Box
                  key={vi.key} data-index={vi.index} ref={rowVirtualizer.measureElement}
                  sx={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${vi.start}px)`, pb: 2.5 }}
                >
                  {m.from === 'user'
                    ? <UserBubble msg={m} disabled={thinking} onEdit={(newText) => editUser(m.id, newText)} />
                    : <AgentMessage msg={m} isLast={m.id === lastAgentId} thinking={thinking} onRetry={retryLast} onReview={() => send(reviewPromptFor(m))} onStop={stop} />}
                </Box>
              );
            })}
          </Box>
        )}

        <Stack spacing={2.5}>
          {thinking && messages[messages.length - 1]?.from === 'user' && (
            <Stack direction="row" spacing={1.5} alignItems="center">
              <AgentAvatar />
              <Box sx={(t) => ({
                px: 1.5, py: 1, borderRadius: `${t.studio.radius.sm}px`,
                bgcolor: 'background.paper',
                border: `1px solid ${t.palette.divider}`,
              })}>
                <TypingDots />
              </Box>
            </Stack>
          )}

          {showFollowups && (
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75, pl: 4.5 }}>
              {FOLLOWUPS.map((f) => (
                <Chip key={f} label={f} clickable size="small" onClick={() => send(f)}
                  sx={(t) => ({
                    height: 'auto', py: 0.55, fontSize: fontScale.s74,
                    bgcolor: 'background.paper',
                    border: `1px solid ${t.palette.divider}`,
                    '& .MuiChip-label': { whiteSpace: 'normal' },
                    '&:hover': { borderColor: t.palette.primary.main, bgcolor: `${t.palette.primary.main}0d` },
                    transition: `border-color ${t.studio.motion.fast}, background-color ${t.studio.motion.fast}`,
                  })} />
              ))}
            </Stack>
          )}
        </Stack>
      </Box>

      {/* Composer */}
      <Composer
        draft={draft}
        setDraft={setDraft}
        thinking={thinking}
        planMode={planMode}
        isLive={isLive}
        workMode={workMode}
        slashCommands={slashCommands}
        slashOpen={slashOpen}
        setSlashOpen={setSlashOpen}
        slashMatches={slashMatches}
        mentionMatches={mentionMatches}
        ctxChips={ctxChips}
        ctxNearFull={ctxNearFull}
        uiLang={uiLang}
        onSend={send}
        onStop={stop}
        onRunSlash={runSlash}
        onPickMention={pickMention}
        onChooseMode={chooseMode}
        onNewChat={handleNewChat}
      />

      {/* History popover */}
      <HistoryPopover
        anchorEl={historyAnchor}
        onClose={() => setHistoryAnchor(null)}
        sessions={chatSessions}
        activeId={activeChatId}
        onSelect={(id) => { selectChat(id); setHistoryAnchor(null); }}
        onArchive={archiveChat}
        onRestore={restoreChat}
        onDelete={deleteChat}
      />

      {/* Pre-spend gate dialog */}
      <PreflightDialog
        open={!!pending}
        prompt={pending?.text ?? ''}
        mode={pending?.mode ?? workMode}
        projectId={targetProjectId}
        isLive={isLive}
        onConfirm={confirmPending}
        onCancel={() => setPending(null)}
      />
    </Box>
  );
}

// ── Agent avatar ──────────────────────────────────────────────────────────────
function AgentAvatar({ size = 26 }: { size?: number }) {
  return (
    <Avatar sx={(t) => ({
      width: size, height: size, flexShrink: 0,
      bgcolor: t.palette.background.paper,
      border: `1px solid ${t.palette.divider}`,
      boxShadow: '0 1px 2px rgba(24,22,20,0.04)',
      fontSize: size * 0.45, fontWeight: 700,
    })}>
      <LogoMark size={Math.max(18, size - 8)} />
    </Avatar>
  );
}

// ── Empty state ───────────────────────────────────────────────────────────────
function EmptyState({ onSuggest }: { onSuggest: (s: string) => void }) {
  return (
    <Box sx={{ pt: 1 }}>
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 1.25 }}>
        <AgentAvatar size={30} />
        <Typography variant="h6" sx={{ fontWeight: 700, fontSize: fontScale.s105, letterSpacing: '-0.01em' }}>
          Build something real
        </Typography>
      </Stack>
      <Typography sx={{ fontSize: fontScale.s86, color: 'text.secondary', lineHeight: 1.6, mb: 2 }}>
        Describe your product and I'll plan it, write the code, and drive it through the finisher gates — patches you can review, costs you can see.
      </Typography>
      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
        {SUGGESTIONS.map((s) => (
          <Chip
            key={s} label={s} clickable size="small" onClick={() => onSuggest(s)}
            sx={(t) => ({
              height: 'auto', py: 0.65, fontSize: fontScale.s76,
              bgcolor: 'background.paper',
              border: `1px solid ${t.palette.divider}`,
              '& .MuiChip-label': { whiteSpace: 'normal' },
              '&:hover': { borderColor: t.palette.primary.main, bgcolor: `${t.palette.primary.main}0d` },
              transition: `border-color ${t.studio.motion.fast}, background-color ${t.studio.motion.fast}`,
            })}
          />
        ))}
      </Stack>
      <Typography sx={(t) => ({ mt: 2.5, fontFamily: t.brand.font.mono, fontSize: fontScale.s70, color: 'text.disabled' })}>
        Type <strong>/</strong> for commands · @mention to route to a specialist
      </Typography>
    </Box>
  );
}

// ── Composer ──────────────────────────────────────────────────────────────────
interface ComposerProps {
  draft: string; setDraft: (v: string) => void; thinking: boolean; planMode: boolean; isLive: boolean;
  workMode: WorkMode; slashCommands: SlashCommand[]; slashOpen: boolean; setSlashOpen: (v: boolean) => void;
  slashMatches: SlashCommand[]; mentionMatches: Mentionable[]; ctxChips: { label: string; title: string }[];
  ctxNearFull: boolean; uiLang: Lang;
  onSend: (s?: string) => void; onStop: () => void; onRunSlash: (c: SlashCommand) => void;
  onPickMention: (m: Mentionable) => void; onChooseMode: (m: WorkMode) => void; onNewChat: () => void;
}

function Composer({ draft, setDraft, thinking, planMode, isLive, workMode, slashCommands, slashOpen, setSlashOpen, slashMatches, mentionMatches, ctxChips, ctxNearFull, uiLang, onSend, onStop, onRunSlash, onPickMention, onChooseMode, onNewChat }: ComposerProps) {
  return (
    <Box sx={(t) => ({
      p: 1.25,
      borderTop: `1px solid ${t.palette.divider}`,
      position: 'relative',
      bgcolor: 'background.paper',
    })}>
      {/* Slash command popover */}
      {slashCommands.length > 0 && (
        <Paper elevation={0} sx={(t) => ({
          position: 'absolute', left: 10, right: 10, bottom: '100%', mb: 0.75,
          border: `1px solid ${t.palette.divider}`,
          borderRadius: `${t.studio.radius.sm}px`,
          overflow: 'hidden', backgroundImage: 'none',
          backdropFilter: 'blur(16px)',
        })}>
          <Typography sx={(t) => ({ px: 1.5, pt: 1, pb: 0.5, fontFamily: t.brand.font.mono, fontSize: fontScale.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
            Slash commands
          </Typography>
          {slashCommands.map((c) => (
            <Box key={c.cmd} onClick={() => onRunSlash(c)} sx={(t) => ({ px: 1.5, py: 0.9, cursor: 'pointer', '&:hover': { bgcolor: `${t.palette.primary.main}0d` } })}>
              <Typography component="span" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s80, color: 'primary.main', mr: 1 })}>{c.cmd}</Typography>
              <Typography component="span" sx={{ fontSize: fontScale.s78, color: 'text.secondary' }}>{c.desc}</Typography>
            </Box>
          ))}
        </Paper>
      )}

      {/* @-mention autocomplete */}
      {mentionMatches.length > 0 && (
        <Paper elevation={0} sx={(t) => ({
          position: 'absolute', left: 10, right: 10, bottom: '100%', mb: 0.75,
          border: `1px solid ${t.palette.divider}`,
          borderRadius: `${t.studio.radius.sm}px`,
          overflow: 'hidden', maxHeight: 240, overflowY: 'auto', backgroundImage: 'none',
          backdropFilter: 'blur(16px)',
        })}>
          {mentionMatches.map((m) => (
            <Stack key={m.token} direction="row" spacing={1} alignItems="center" onClick={() => onPickMention(m)} sx={(t) => ({ px: 1.5, py: 0.9, cursor: 'pointer', '&:hover': { bgcolor: `${t.palette.primary.main}0d` } })}>
              <VscRobot size={13} />
              <Typography component="span" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s78, color: 'primary.main' })}>@{m.token}</Typography>
              <Typography component="span" noWrap sx={{ fontSize: fontScale.s74, color: 'text.secondary', minWidth: 0 }}>{m.role}</Typography>
            </Stack>
          ))}
        </Paper>
      )}

      {/* Context chips */}
      {ctxChips.length > 0 && (
        <Stack direction="row" spacing={0.5} sx={{ mb: 0.75, flexWrap: 'wrap', gap: 0.5 }}>
          {ctxChips.map((c) => (
            <Tooltip key={c.label} title={c.title} arrow>
              <Chip size="small" label={c.label} sx={(t) => ({ height: 18, fontSize: fontScale.s60, fontFamily: t.brand.font.mono, bgcolor: `${t.palette.primary.main}14`, color: 'primary.main', border: `1px solid ${t.palette.primary.main}33` })} />
            </Tooltip>
          ))}
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s60, color: 'text.disabled', alignSelf: 'center' })}>grounding this chat</Typography>
        </Stack>
      )}

      {/* Mode selector */}
      <ToggleButtonGroup
        exclusive size="small" value={workMode}
        onChange={(_, next: WorkMode | null) => { if (next) onChooseMode(next); }}
        sx={(t) => ({
          mb: 0.75, display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
          border: `1px solid ${t.palette.divider}`,
          borderRadius: `${t.studio.radius.sm}px`, overflow: 'hidden',
          bgcolor: t.palette.surfaceHover,
          '& .MuiToggleButtonGroup-grouped': { m: 0, border: 0, borderRadius: 0 },
        })}
      >
        {MODES.map(({ id, label, tip, Icon }) => (
          <ToggleButton key={id} value={id} aria-label={`${label} mode`}
            sx={(t) => ({
              minWidth: 0, px: 0.5, py: 0.6, gap: 0.4, color: 'text.secondary',
              '&.Mui-selected': {
                color: 'text.primary',
                bgcolor: 'background.paper',
                fontWeight: 700,
                boxShadow: '0 1px 2px rgba(24,22,20,0.05)',
              },
              transition: `color ${t.studio.motion.fast}, background-color ${t.studio.motion.fast}`,
            })}
          >
            <Tooltip title={tip} arrow>
              <Stack component="span" direction="row" spacing={0.4} alignItems="center" sx={{ minWidth: 0 }}>
                <Icon size={12} />
                <Typography component="span" noWrap sx={{ fontSize: fontScale.s68, fontWeight: 'inherit' }}>{label}</Typography>
              </Stack>
            </Tooltip>
          </ToggleButton>
        ))}
      </ToggleButtonGroup>

      {/* Context near-full warning */}
      {ctxNearFull && (
        <Stack direction="row" alignItems="center" spacing={1} sx={(t) => ({ mb: 0.75, px: 1.25, py: 0.75, borderRadius: `${t.studio.radius.sm}px`, border: `1px solid ${t.palette.warning.main}`, bgcolor: `${t.palette.warning.main}14` })}>
          <Typography sx={{ flex: 1, fontSize: fontScale.s74, color: 'warning.main', lineHeight: 1.4 }}>
            {t('ctxFull', uiLang)}
          </Typography>
          <Chip size="small" clickable color="warning" variant="outlined" label={t('newChat', uiLang)} onClick={onNewChat} sx={{ fontSize: fontScale.s70, flexShrink: 0 }} />
        </Stack>
      )}

      {/* Input area */}
      <Box sx={(t) => ({
        border: `1px solid ${t.palette.divider}`,
        borderRadius: `${t.studio.radius.sm}px`,
        bgcolor: '#FFFEFC',
        p: 1.25,
        transition: `border-color ${t.studio.motion.fast}, box-shadow ${t.studio.motion.fast}`,
        '&:focus-within': {
          borderColor: t.palette.primary.main,
          boxShadow: `0 0 0 2px ${t.palette.primary.main}18`,
        },
      })}>
        <InputBase
          fullWidth multiline maxRows={5}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape' && thinking) { e.preventDefault(); onStop(); return; }
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              if (mentionMatches.length >= 1) { onPickMention(mentionMatches[0]!); return; }
              if (slashMatches.length === 1) onRunSlash(slashMatches[0]!);
              else onSend();
            }
          }}
          placeholder="Ask, or @mention an agent · / for commands"
          sx={{ fontSize: fontScale.s90, px: 0.5, lineHeight: 1.6 }}
        />
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 0.75 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ minWidth: 0 }}>
            {/* Connection status */}
            <Typography noWrap sx={(t) => ({
              fontFamily: t.brand.font.mono, fontSize: fontScale.s70,
              color: isLive ? 'success.main' : 'text.disabled',
            })}>
              {isLive ? '● connected' : '○ offline'}
            </Typography>
            {/* Slash toggle */}
            <Tooltip title="Show slash commands" arrow>
              <IconButton size="small" aria-label="Show slash commands" onClick={() => setSlashOpen(!slashOpen)}
                sx={(t) => ({
                  color: slashOpen ? 'primary.main' : 'text.secondary',
                  border: `1px solid ${slashOpen ? t.palette.primary.main : t.palette.divider}`,
                  width: 22, height: 22,
                  transition: `border-color ${t.studio.motion.fast}, color ${t.studio.motion.fast}`,
                })}>
                <VscTerminal size={11} />
              </IconButton>
            </Tooltip>
            {/* Preflight indicator */}
            <Tooltip title={planMode ? 'Preflight is armed for this mode' : 'This mode dispatches without the pre-spend gate'} arrow>
              <Stack direction="row" alignItems="center" spacing={0.4}
                sx={(t) => ({
                  px: 0.75, py: 0.2, borderRadius: 99,
                  border: `1px solid ${planMode ? t.palette.primary.main : t.palette.divider}`,
                  color: planMode ? 'primary.main' : 'text.disabled',
                  userSelect: 'none', cursor: 'default',
                  bgcolor: planMode ? `${t.palette.primary.main}14` : 'transparent',
                  transition: `all ${t.studio.motion.fast}`,
                })}
              >
                <VscShield size={10} />
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s64, fontWeight: 600 })}>
                  {planMode ? 'preflight' : 'direct'}
                </Typography>
              </Stack>
            </Tooltip>
          </Stack>
          {/* Send / stop button */}
          {thinking ? (
            <Tooltip title="Stop generating (Esc)" arrow>
              <IconButton onClick={onStop} size="small" aria-label="Stop"
                sx={(t) => ({
                  color: 'text.primary',
                  border: `1px solid ${t.palette.divider}`,
                  width: 28, height: 28,
                  '&:hover': { borderColor: t.palette.error.main, color: 'error.main' },
                })}>
                <VscDebugStop size={13} />
              </IconButton>
            </Tooltip>
          ) : (
            <Tooltip title="Send (Enter)" arrow>
              <Box component="span">
                <IconButton onClick={() => onSend()} disabled={!draft.trim()} size="small" aria-label="Send"
                  sx={(t) => ({
                    color: 'common.white', bgcolor: 'text.primary', backgroundImage: 'none',
                    width: 28, height: 28,
                    transition: `transform ${t.studio.motion.fast}, box-shadow ${t.studio.motion.fast}`,
                    '&:hover': { transform: 'scale(1.03)', bgcolor: 'text.primary', boxShadow: '0 6px 14px rgba(24,22,20,0.16)' },
                    '&.Mui-disabled': { backgroundImage: 'none', bgcolor: 'action.disabledBackground', color: 'text.disabled' },
                  })}>
                  <VscSend size={13} />
                </IconButton>
              </Box>
            </Tooltip>
          )}
        </Stack>
      </Box>
    </Box>
  );
}

// ── Agent message ─────────────────────────────────────────────────────────────
function AgentMessage({ msg, isLast, thinking, onRetry, onReview, onStop }: {
  msg: TrustChatMsg; isLast: boolean; thinking: boolean; onRetry: () => void; onReview: () => void; onStop: () => void;
}) {
  return (
    <Stack direction="row" spacing={1.25} alignItems="flex-start">
      <AgentAvatar />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        {/* Header row */}
        <Stack direction="row" spacing={0.75} alignItems="center" sx={{ mb: 0.75 }}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s68, color: 'text.disabled', letterSpacing: '0.04em' })}>
            Orchestrator
          </Typography>
          {msg.mode && (
            <Chip size="small" label={msg.mode}
              sx={(t) => ({
                height: 16, fontFamily: t.brand.font.mono, fontSize: fontScale.s58,
                color: 'text.secondary', bgcolor: 'action.hover',
                border: `1px solid ${t.palette.divider}`,
              })} />
          )}
        </Stack>

        {/* Reasoning (collapsed by default) */}
        {msg.thinking && <ReasoningBlock text={msg.thinking} />}

        {/* Tool steps */}
        {msg.steps && msg.steps.length > 0 && <ToolSteps steps={msg.steps} />}

        {/* Content */}
        {msg.text ? <Markdown>{msg.text}</Markdown> : <TypingDots />}

        {/* Evidence footer */}
        <EvidenceFooter msg={msg} />

        {/* Action row */}
        {msg.text && (
          <Stack direction="row" spacing={0.25} sx={{ mt: 0.5, opacity: 0.5, '&:hover': { opacity: 1 }, transition: 'opacity 180ms' }}>
            <Tooltip title="Copy" arrow>
              <IconButton size="small" onClick={() => void navigator.clipboard?.writeText(msg.text)} sx={{ color: 'text.secondary', p: 0.35 }}>
                <VscCopy size={12} />
              </IconButton>
            </Tooltip>
            <Tooltip title="Review this answer" arrow>
              <IconButton size="small" onClick={onReview} sx={{ color: 'text.secondary', p: 0.35 }}>
                <VscPreview size={12} />
              </IconButton>
            </Tooltip>
            {isLast && !thinking && (
              <Tooltip title="Retry this reply" arrow>
                <IconButton size="small" onClick={onRetry} sx={{ color: 'text.secondary', p: 0.35 }}>
                  <VscRefresh size={12} />
                </IconButton>
              </Tooltip>
            )}
            {isLast && thinking && (
              <Tooltip title="Stop generating" arrow>
                <IconButton size="small" onClick={onStop} sx={{ color: 'text.secondary', p: 0.35 }}>
                  <VscDebugStop size={12} />
                </IconButton>
              </Tooltip>
            )}
          </Stack>
        )}
      </Box>
    </Stack>
  );
}

// ── Tool steps ────────────────────────────────────────────────────────────────
function ToolSteps({ steps }: { steps: string[] }) {
  const [open, setOpen] = useState(false);
  const visible = open ? steps : steps.slice(-2);
  return (
    <Box sx={(t) => ({
      mb: 1, p: 1, borderRadius: `${t.studio.radius.sm}px`,
      bgcolor: `${t.palette.primary.main}08`,
      border: `1px solid ${t.palette.primary.main}20`,
    })}>
      {visible.map((s, i) => (
        <Stack key={`${s}-${i}`} direction="row" spacing={0.75} alignItems="center" sx={{ mb: i < visible.length - 1 ? 0.4 : 0 }}>
          <Box component="span" sx={{ color: 'primary.main', fontSize: fontScale.s80, lineHeight: 1 }}>→</Box>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s72, color: 'text.secondary', lineHeight: 1.4 })}>{s}</Typography>
        </Stack>
      ))}
      {steps.length > 2 && (
        <Typography onClick={() => setOpen((v) => !v)} sx={{ fontSize: fontScale.s68, color: 'primary.main', cursor: 'pointer', mt: 0.5 }}>
          {open ? 'Show fewer steps' : `+${steps.length - 2} more steps`}
        </Typography>
      )}
    </Box>
  );
}

// ── Evidence footer ───────────────────────────────────────────────────────────
function EvidenceFooter({ msg }: { msg: TrustChatMsg }) {
  const files = msg.filesExtracted ?? [];
  const routed = msg.routedTo ?? [];
  const steps = msg.steps ?? [];
  const risks = msg.riskHints ?? [];
  const hasEvidence = files.length > 0 || routed.length > 0 || steps.length > 0 || risks.length > 0 || typeof msg.costUSD === 'number';
  if (!hasEvidence) return null;

  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mt: 0.75 }}>
      {files.length > 0 && (
        <Tooltip title={files.slice(0, 8).join('\n')} arrow>
          <Chip size="small" label={`${files.length} file${files.length === 1 ? '' : 's'} extracted`}
            sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, bgcolor: `${t.palette.success.main}14`, color: 'success.main', border: `1px solid ${t.palette.success.main}33` })} />
        </Tooltip>
      )}
      {steps.length > 0 && (
        <Tooltip title={steps.slice(-8).join('\n')} arrow>
          <Chip size="small" label={`${steps.length} tool step${steps.length === 1 ? '' : 's'}`}
            sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, bgcolor: 'action.hover', color: 'text.secondary' })} />
        </Tooltip>
      )}
      {routed.length > 0 && (
        <Tooltip title={routed.join(', ')} arrow>
          <Chip size="small" icon={<VscRobot size={10} />} label={`${routed.length} agent${routed.length === 1 ? '' : 's'} routed`}
            sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, bgcolor: `${t.palette.secondary.main}14`, color: 'secondary.main', border: `1px solid ${t.palette.secondary.main}33`, '& .MuiChip-icon': { ml: 0.5 } })} />
        </Tooltip>
      )}
      {typeof msg.costUSD === 'number' && (
        <Chip size="small" label={`cost ${formatUSD(msg.costUSD)}`}
          sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, bgcolor: 'action.hover', color: 'text.secondary' })} />
      )}
      {risks.map((r) => (
        <Chip key={r} size="small" icon={<VscShield size={10} />} label={r}
          sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: fontScale.s58, bgcolor: `${t.palette.warning.main}14`, color: 'text.secondary', border: `1px solid ${t.palette.warning.main}33`, '& .MuiChip-icon': { ml: 0.5, color: 'warning.main' } })} />
      ))}
    </Stack>
  );
}

// ── User bubble ───────────────────────────────────────────────────────────────
function UserBubble({ msg, disabled, onEdit }: { msg: ChatMsg; disabled: boolean; onEdit: (text: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(msg.text);
  if (editing) {
    return (
      <Box sx={{ alignSelf: 'flex-end', width: '92%' }}>
        <TextField
          value={val} autoFocus multiline fullWidth size="small"
          onChange={(e) => setVal(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); setEditing(false); onEdit(val); }
            if (e.key === 'Escape') { setEditing(false); setVal(msg.text); }
          }}
          sx={{ '& .MuiInputBase-input': { fontSize: fontScale.s90 } }}
        />
        <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{ mt: 0.5 }}>
          <Chip size="small" clickable label="Cancel" onClick={() => { setEditing(false); setVal(msg.text); }}
            sx={{ height: 22, fontSize: fontScale.s70, bgcolor: 'action.hover' }} />
          <Chip size="small" clickable label="Send" onClick={() => { setEditing(false); onEdit(val); }}
            sx={{ height: 22, fontSize: fontScale.s70, color: 'common.white', bgcolor: 'text.primary' }} />
        </Stack>
      </Box>
    );
  }
  return (
    <Stack direction="row" spacing={0.5} justifyContent="flex-end" alignItems="flex-start" sx={{ '&:hover .if-edit': { opacity: 1 } }}>
      <Tooltip title="Edit & resend" arrow>
        <IconButton size="small" disabled={disabled} className="if-edit" onClick={() => { setVal(msg.text); setEditing(true); }}
          sx={{ opacity: 0, color: 'text.disabled', p: 0.35, mt: 0.25 }}>
          <VscEdit size={11} />
        </IconButton>
      </Tooltip>
      <Box sx={(t) => ({
        bgcolor: 'surfaceHover',
        border: `1px solid ${t.palette.divider}`,
        borderRadius: `${t.studio.radius.sm}px`,
        px: 1.75, py: 1, maxWidth: '85%',
      })}>
        {msg.routedTo && msg.routedTo.length > 0 && (
          <Stack direction="row" spacing={0.5} sx={{ mb: 0.5, flexWrap: 'wrap', gap: 0.5 }}>
            {msg.routedTo.map((r) => (
              <Chip key={r} size="small" icon={<VscRobot size={10} />} label={r}
                sx={(t) => ({ height: 17, fontSize: fontScale.s58, fontFamily: t.brand.font.mono, bgcolor: 'background.paper', '& .MuiChip-icon': { ml: 0.5 } })} />
            ))}
          </Stack>
        )}
        <Typography sx={{ fontSize: fontScale.s90, whiteSpace: 'pre-wrap', lineHeight: 1.6 }}>{msg.text}</Typography>
      </Box>
    </Stack>
  );
}

// ── Reasoning block ───────────────────────────────────────────────────────────
function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <Box sx={{ mb: 1 }}>
      <Stack direction="row" alignItems="center" spacing={0.75} onClick={() => setOpen((v) => !v)}
        sx={(t) => ({ cursor: 'pointer', color: 'text.secondary', '&:hover': { color: 'text.primary' }, transition: `color ${t.studio.motion.fast}` })}>
        <VscLightbulb size={12} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s68, letterSpacing: '0.04em' })}>Reasoning</Typography>
        {open ? <VscChevronDown size={11} /> : <VscChevronRight size={11} />}
      </Stack>
      {open && (
        <Box sx={(t) => ({
          mt: 0.5, p: 1, borderRadius: `${t.studio.radius.sm}px`,
          borderLeft: `2px solid ${t.palette.primary.main}`,
          bgcolor: `${t.palette.primary.main}08`,
        })}>
          <Typography sx={{ fontSize: fontScale.s78, color: 'text.secondary', whiteSpace: 'pre-wrap', lineHeight: 1.55 }}>{text}</Typography>
        </Box>
      )}
    </Box>
  );
}

// ── Chat header ───────────────────────────────────────────────────────────────
function ChatHeader({ title, onNew, onOpenHistory, onRename }: {
  title: string; onNew: () => void; onOpenHistory: (el: HTMLElement) => void; onRename: (t: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(title);
  useEffect(() => { setVal(title); }, [title]);
  const commit = () => { setEditing(false); if (val.trim() && val.trim() !== title) onRename(val.trim()); };
  return (
    <Stack data-testid="chat-header" direction="row" alignItems="center" spacing={0.5} sx={(t) => ({
      height: 48,
      px: 1.5,
      py: 0,
      borderBottom: `1px solid ${t.palette.divider}`,
      bgcolor: 'background.paper',
    })}>
      <Box sx={(t) => ({
        width: 6, height: 6, borderRadius: 99, flexShrink: 0,
        bgcolor: t.palette.primary.main,
      })} />
      {editing ? (
        <ClickAwayListener onClickAway={commit}>
          <TextField
            value={val} autoFocus variant="standard"
            onChange={(e) => setVal(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') commit(); if (e.key === 'Escape') { setEditing(false); setVal(title); } }}
            sx={{ flex: 1, '& .MuiInput-input': { fontSize: fontScale.s85, py: 0.2 } }}
          />
        </ClickAwayListener>
      ) : (
        <>
          <Typography noWrap sx={{ flex: 1, fontSize: fontScale.s85, fontWeight: 600 }}>{title}</Typography>
          <Tooltip title="Rename chat" arrow>
            <IconButton size="small" onClick={() => setEditing(true)} sx={{ color: 'text.disabled', p: 0.4 }}>
              <VscEdit size={12} />
            </IconButton>
          </Tooltip>
        </>
      )}
      <Tooltip title="Chat history" arrow>
        <IconButton size="small" onClick={(e) => onOpenHistory(e.currentTarget)} sx={{ color: 'text.secondary', p: 0.4 }}>
          <VscHistory size={14} />
        </IconButton>
      </Tooltip>
      <Tooltip title="New chat" arrow>
        <IconButton size="small" onClick={onNew} sx={{ color: 'text.secondary', p: 0.4 }}>
          <VscAdd size={14} />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

// ── History popover ───────────────────────────────────────────────────────────
function HistoryPopover({ anchorEl, onClose, sessions, activeId, onSelect, onArchive, onRestore, onDelete }: {
  anchorEl: HTMLElement | null; onClose: () => void;
  sessions: ReturnType<typeof useStudio.getState>['chatSessions']; activeId: string | null;
  onSelect: (id: string) => void; onArchive: (id: string) => void; onRestore: (id: string) => void; onDelete: (id: string) => void;
}) {
  const [showArchived, setShowArchived] = useState(false);
  const live = sessions.filter((c) => !c.archived).sort((a, b) => b.updatedAt - a.updatedAt);
  const archived = sessions.filter((c) => c.archived).sort((a, b) => b.updatedAt - a.updatedAt);
  return (
    <Popover
      open={!!anchorEl} anchorEl={anchorEl} onClose={onClose}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      slotProps={{ paper: { sx: { width: 320, maxHeight: 460, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
    >
      <Typography sx={(t) => ({ px: 2, pt: 1.5, pb: 1, fontFamily: t.brand.font.mono, fontSize: fontScale.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
        Chats
      </Typography>
      {live.length === 0 ? (
        <Typography sx={{ px: 2, pb: 1.5, fontSize: fontScale.s80, color: 'text.secondary' }}>No conversations yet.</Typography>
      ) : (
        live.map((c) => (
          <SessionRow key={c.id} session={c} active={c.id === activeId}
            onClick={() => onSelect(c.id)} action={<VscArchive size={12} />}
            actionTitle="Archive" onAction={() => onArchive(c.id)} />
        ))
      )}
      {archived.length > 0 && (
        <Box sx={{ borderTop: 1, borderColor: 'divider', mt: 0.5 }}>
          <MenuItem onClick={() => setShowArchived((v) => !v)} sx={{ py: 0.75 }}>
            {showArchived ? <VscChevronDown size={12} /> : <VscChevronRight size={12} />}
            <Typography sx={{ ml: 1, fontSize: fontScale.s78, color: 'text.secondary' }}>Archived ({archived.length})</Typography>
          </MenuItem>
          {showArchived && archived.map((c) => (
            <SessionRow key={c.id} session={c} active={false} muted
              onClick={() => onRestore(c.id)} action={<VscTrash size={12} />}
              actionTitle="Delete permanently" onAction={() => onDelete(c.id)} />
          ))}
        </Box>
      )}
    </Popover>
  );
}

function SessionRow({ session, active, muted, onClick, action, actionTitle, onAction }: {
  session: ReturnType<typeof useStudio.getState>['chatSessions'][number];
  active: boolean; muted?: boolean; onClick: () => void; action: React.ReactNode; actionTitle: string; onAction: () => void;
}) {
  return (
    <Stack direction="row" alignItems="center" sx={(t) => ({
      px: 1, mx: 1, borderRadius: `${t.studio.radius.sm}px`,
      bgcolor: active ? `${t.palette.primary.main}14` : 'transparent',
      '&:hover': { bgcolor: active ? `${t.palette.primary.main}1a` : 'action.hover' },
      transition: `background-color ${t.studio.motion.fast}`,
    })}>
      <Box onClick={onClick} sx={{ flex: 1, minWidth: 0, py: 0.85, px: 1, cursor: 'pointer', opacity: muted ? 0.65 : 1 }}>
        <Typography noWrap sx={{ fontSize: fontScale.s82, fontWeight: active ? 600 : 400 }}>{session.title}</Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s60, color: 'text.disabled' })}>
          {session.messages.length} msg · {formatRelativeTime(session.updatedAt)}{muted ? ' · click to restore' : ''}
        </Typography>
      </Box>
      <Tooltip title={actionTitle} arrow>
        <IconButton size="small" onClick={(e) => { e.stopPropagation(); onAction(); }} sx={{ color: 'text.disabled', p: 0.4 }}>
          {action}
        </IconButton>
      </Tooltip>
    </Stack>
  );
}
