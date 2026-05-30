import { useMemo, useState } from 'react';
import Editor, { type BeforeMount } from '@monaco-editor/react';
import { strToU8, zipSync } from 'fflate';
import { Box, Chip, Divider, IconButton, LinearProgress, Stack, Tooltip, Typography } from '@mui/material';
import {
  VscChevronDown,
  VscChevronRight,
  VscClose,
  VscCloudDownload,
  VscCopy,
  VscEdit,
  VscEllipsis,
  VscFile,
  VscFileCode,
  VscFileMedia,
  VscFileZip,
  VscFolder,
  VscFolderOpened,
  VscGear,
  VscJson,
  VscMarkdown,
  VscNewFile,
  VscPackage,
  VscPass,
  VscRefresh,
  VscShield,
  VscSymbolNamespace,
  VscTrash,
} from 'react-icons/vsc';
import { LogoMark } from '../components/LogoMark';
import { useStudio, type GeneratedFile } from '../store';
import { toast } from '@ironflyer/ui-web/fx';
import { text as fontScale } from '@ironflyer/design-tokens/brand';
import { EditorConfirmDialog } from './editor-dialogs/EditorConfirmDialog';
import { EditorFileDialog, type EditorFileDialogMode } from './editor-dialogs/EditorFileDialog';

// Professional editor usage kept here intentionally, per the current experiment.
// The full branded Eclipse Theia workspace can be restored by re-importing:
//   import { IdeFrame } from '../components/IdeFrame';
//   import { useLiveProjectId } from '../hooks/useLiveProjectId';
// and rendering:
//   <IdeFrame projectId={projectId} />
// For now the Code tab runs a fast local Monaco surface with a VS-style file tree.

type FileTreeNode = {
  name: string;
  path: string;
  kind: 'file' | 'dir';
  children?: FileTreeNode[];
};

type EditorDialogState =
  | { kind: EditorFileDialogMode }
  | { kind: 'delete' }
  | null;

const SAMPLE_FILES: GeneratedFile[] = [
  {
    path: 'README.md',
    rev: 1,
    content: `# Northwind Checkout

Stripe checkout + webhook flow, scaffolded by Ironflyer and finished through the gates.

## Overview

This project demonstrates a full Stripe checkout integration with webhook handling and gated deployments.

## Key Features

- Checkout session creation
- Webhook signature verification
- Order fulfillment
- Security gates & automated tests

## Getting Started

1. Install dependencies

\`\`\`bash
pnpm install
\`\`\`

2. Copy env

\`\`\`bash
cp .env.example .env.local
\`\`\`

3. Run dev server

\`\`\`bash
pnpm dev
\`\`\`
`,
  },
  {
    path: 'src/api/webhook.ts',
    rev: 1,
    content: `import { headers } from 'next/headers';
import Stripe from 'stripe';

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY ?? '');

export async function POST(request: Request) {
  const body = await request.text();
  const signature = headers().get('stripe-signature');

  if (!signature) {
    return Response.json({ error: 'missing signature' }, { status: 400 });
  }

  const event = stripe.webhooks.constructEvent(
    body,
    signature,
    process.env.STRIPE_WEBHOOK_SECRET ?? '',
  );

  if (event.type === 'checkout.session.completed') {
    // Fulfill the order and close the deploy gate.
  }

  return Response.json({ received: true });
}
`,
  },
  {
    path: 'src/routes/checkout.ts',
    rev: 1,
    content: `import Stripe from 'stripe';

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY ?? '');

export async function createCheckoutSession(priceId: string) {
  return stripe.checkout.sessions.create({
    mode: 'payment',
    line_items: [{ price: priceId, quantity: 1 }],
    success_url: '/success',
    cancel_url: '/checkout',
  });
}
`,
  },
  {
    path: 'src/routes/index.ts',
    rev: 1,
    content: `export { createCheckoutSession } from './checkout';
`,
  },
  {
    path: 'src/lib/stripe.ts',
    rev: 1,
    content: `import Stripe from 'stripe';

export const stripe = new Stripe(process.env.STRIPE_SECRET_KEY ?? '', {
  apiVersion: '2025-04-30.basil',
});
`,
  },
  {
    path: '.env.local',
    rev: 1,
    content: `STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
NEXT_PUBLIC_APP_URL=http://localhost:3000
`,
  },
  {
    path: 'package.json',
    rev: 1,
    content: `{
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "test": "vitest run"
  },
  "dependencies": {
    "next": "15.3.0",
    "react": "19.0.0",
    "stripe": "^17.7.0"
  }
}
`,
  },
  {
    path: 'tsconfig.json',
    rev: 1,
    content: `{
  "compilerOptions": {
    "target": "ES2022",
    "strict": true,
    "moduleResolution": "bundler"
  }
}
`,
  },
];

const ACTIVE_AGENTS = [
  { name: 'Planner', status: 'Planning', tone: 'info', progress: 76 },
  { name: 'Code Generator', status: 'Running', tone: 'primary', progress: 58 },
  { name: 'Security Review', status: 'Waiting', tone: 'warning', progress: 22 },
  { name: 'Deploy Agent', status: 'Ready', tone: 'success', progress: 100 },
] as const;

const GATES = [
  { name: 'Build', status: 'Passed', tone: 'success' },
  { name: 'Tests', status: 'Passed', tone: 'success' },
  { name: 'Security', status: 'Passed', tone: 'success' },
  { name: 'Performance', status: 'Passed', tone: 'success' },
  { name: 'Cost Guard', status: 'Open', tone: 'warning' },
] as const;

const ACTIVITY = [
  { text: 'You edited README.md', time: '2m ago', tone: 'info' },
  { text: 'Webhook endpoint added', time: '12m ago', tone: 'info' },
  { text: 'Security gate passed', time: '25m ago', tone: 'success' },
  { text: 'All tests passed', time: '32m ago', tone: 'success' },
] as const;

function languageForPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'md': return 'markdown';
    case 'json': return 'json';
    case 'ts':
    case 'tsx': return 'typescript';
    case 'js':
    case 'jsx': return 'javascript';
    case 'css': return 'css';
    case 'html': return 'html';
    case 'env':
    case 'local': return 'shell';
    default: return 'plaintext';
  }
}

function fileIcon(path: string) {
  const name = path.split('/').pop()?.toLowerCase() ?? path.toLowerCase();
  const ext = name.split('.').pop() ?? '';
  if (name === 'package.json') return { icon: <VscPackage />, color: '#2f7bff' };
  if (name === 'tsconfig.json') return { icon: <VscGear />, color: '#4c79ff' };
  if (name.startsWith('.env')) return { icon: <VscShield />, color: '#10b981' };
  if (ext === 'md') return { icon: <VscMarkdown />, color: '#2563eb' };
  if (ext === 'json') return { icon: <VscJson />, color: '#f59e0b' };
  if (['ts', 'tsx', 'js', 'jsx'].includes(ext)) return { icon: <VscFileCode />, color: '#0284c7' };
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg'].includes(ext)) return { icon: <VscFileMedia />, color: '#ec4899' };
  return { icon: <VscFile />, color: '#64748b' };
}

function fileBadge(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() ?? '';
  if (['ts', 'tsx'].includes(ext)) return 'TS';
  if (['js', 'jsx'].includes(ext)) return 'JS';
  if (ext === 'md') return 'MD';
  if (ext === 'json') return '{}';
  if (path.startsWith('.env')) return 'ENV';
  return '';
}

function starterContent(path: string): string {
  const language = languageForPath(path);
  if (language === 'json') return '{\n  \n}\n';
  if (language === 'markdown') return `# ${path.split('/').pop()?.replace(/\.[^.]+$/, '') ?? 'New file'}\n`;
  if (language === 'typescript') return 'export {};\n';
  if (language === 'javascript') return 'export {};\n';
  if (language === 'css') return ':root {\n  color-scheme: light;\n}\n';
  if (language === 'html') return '<!doctype html>\n<html>\n  <body></body>\n</html>\n';
  return '';
}

function downloadText(path: string, content: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = path.split('/').pop() || 'file.txt';
  a.click();
  URL.revokeObjectURL(url);
}

function projectZipName(projectName: string): string {
  const safe = projectName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return `${safe || 'ironflyer-project'}.zip`;
}

function safeZipPath(path: string): string | null {
  const normalized = path
    .replace(/\\/g, '/')
    .split('/')
    .filter((part) => part && part !== '.' && part !== '..')
    .join('/');
  return normalized && !normalized.startsWith('/') ? normalized : null;
}

function downloadProjectZip(projectName: string, files: GeneratedFile[]) {
  const entries: Record<string, Uint8Array> = {};
  for (const file of files) {
    const path = safeZipPath(file.path);
    if (path) entries[path] = strToU8(file.content);
  }
  const zipped = zipSync(entries, { level: 6 });
  const blob = new Blob([zipped], { type: 'application/zip' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = projectZipName(projectName);
  a.click();
  URL.revokeObjectURL(url);
}

function buildTree(files: GeneratedFile[]): FileTreeNode[] {
  const root: FileTreeNode[] = [];
  const ensureDir = (siblings: FileTreeNode[], name: string, path: string) => {
    let node = siblings.find((item) => item.kind === 'dir' && item.name === name);
    if (!node) {
      node = { name, path, kind: 'dir', children: [] };
      siblings.push(node);
    }
    return node;
  };

  for (const file of files) {
    const parts = file.path.split('/').filter(Boolean);
    let siblings = root;
    let currentPath = '';
    parts.forEach((part, index) => {
      currentPath = currentPath ? `${currentPath}/${part}` : part;
      if (index === parts.length - 1) {
        siblings.push({ name: part, path: file.path, kind: 'file' });
      } else {
        const dir = ensureDir(siblings, part, currentPath);
        siblings = dir.children ?? [];
      }
    });
  }

  const sort = (nodes: FileTreeNode[]): FileTreeNode[] =>
    nodes
      .map((node) => node.kind === 'dir' ? { ...node, children: sort(node.children ?? []) } : node)
      .sort((a, b) => {
        if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });

  return sort(root);
}

function TreeRow({
  node,
  depth,
  selectedPath,
  openDirs,
  onToggle,
  onSelect,
}: {
  node: FileTreeNode;
  depth: number;
  selectedPath: string;
  openDirs: Set<string>;
  onToggle: (path: string) => void;
  onSelect: (path: string) => void;
}) {
  const isOpen = openDirs.has(node.path);
  const isSelected = node.kind === 'file' && node.path === selectedPath;
  const icon = fileIcon(node.path);

  return (
    <>
      <Box
        component="button"
        type="button"
        onClick={() => node.kind === 'dir' ? onToggle(node.path) : onSelect(node.path)}
        sx={(t) => ({
          width: '100%',
          height: 28,
          display: 'grid',
          gridTemplateColumns: '16px 16px minmax(0, 1fr) auto',
          alignItems: 'center',
          gap: 0.7,
          pl: `${10 + depth * 16}px`,
          pr: 1,
          border: 0,
          borderRadius: 1,
          bgcolor: isSelected ? 'action.selected' : 'transparent',
          color: isSelected ? 'primary.main' : 'text.secondary',
          cursor: 'pointer',
          fontFamily: t.brand.font.body,
          fontSize: fontScale.s72,
          textAlign: 'left',
          '&:hover': { bgcolor: 'action.hover' },
        })}
      >
        <Box sx={{ display: 'inline-flex', color: 'text.disabled' }}>
          {node.kind === 'dir' ? (isOpen ? <VscChevronDown /> : <VscChevronRight />) : null}
        </Box>
        <Box sx={{ display: 'inline-flex', color: node.kind === 'dir' ? '#4f83ff' : icon.color, fontSize: 15 }}>
          {node.kind === 'dir' ? (isOpen ? <VscFolderOpened /> : <VscFolder />) : icon.icon}
        </Box>
        <Typography noWrap sx={{ fontSize: fontScale.s72, fontWeight: isSelected ? 650 : 500 }}>
          {node.name}
        </Typography>
        {node.kind === 'file' && fileBadge(node.path) && (
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s58, color: icon.color, fontWeight: 700 })}>
            {fileBadge(node.path)}
          </Typography>
        )}
      </Box>
      {node.kind === 'dir' && isOpen && node.children?.map((child) => (
        <TreeRow
          key={child.path}
          node={child}
          depth={depth + 1}
          selectedPath={selectedPath}
          openDirs={openDirs}
          onToggle={onToggle}
          onSelect={onSelect}
        />
      ))}
    </>
  );
}

function Explorer({
  files,
  selectedPath,
  onSelect,
  onCreateFile,
  onRenameFile,
  onDeleteFile,
  onCopyPath,
  onDownloadFile,
  onDownloadProject,
  onCloseEditor,
}: {
  files: GeneratedFile[];
  selectedPath: string;
  onSelect: (path: string) => void;
  onCreateFile: () => void;
  onRenameFile: () => void;
  onDeleteFile: () => void;
  onCopyPath: () => void;
  onDownloadFile: () => void;
  onDownloadProject: () => void;
  onCloseEditor: () => void;
}) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [openDirs, setOpenDirs] = useState<Set<string>>(() => new Set(['src', 'src/api', 'src/routes', 'src/lib']));
  const selectedName = selectedPath.split('/').pop() ?? selectedPath;

  const toggle = (path: string) => {
    setOpenDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  return (
    <Box sx={(t) => ({
      width: { xs: 210, xl: 270 },
      minWidth: { xs: 210, xl: 270 },
      borderRight: `1px solid ${t.palette.divider}`,
      bgcolor: 'rgba(248, 250, 255, 0.66)',
      display: 'flex',
      flexDirection: 'column',
      minHeight: 0,
    })}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1.5, height: 48 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s62, color: 'text.secondary', textTransform: 'uppercase', fontWeight: 700 })}>
          Explorer
        </Typography>
        <Stack direction="row" alignItems="center" spacing={0.6} sx={{ color: 'text.secondary' }}>
          <Tooltip title="New file">
            <IconButton size="small" onClick={onCreateFile} aria-label="New file" sx={{ width: 24, height: 24, color: 'text.secondary' }}>
              <VscNewFile size={14} />
            </IconButton>
          </Tooltip>
          <Tooltip title="Rename file">
            <IconButton size="small" onClick={onRenameFile} aria-label="Rename file" sx={{ width: 24, height: 24, color: 'text.secondary' }}>
              <VscEdit size={13} />
            </IconButton>
          </Tooltip>
          <Tooltip title="Delete file">
            <IconButton size="small" onClick={onDeleteFile} aria-label="Delete file" sx={{ width: 24, height: 24, color: 'text.secondary' }}>
              <VscTrash size={13} />
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>
      <Divider />
      <Box sx={{ p: 1, borderBottom: 1, borderColor: 'divider' }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s60, color: 'text.primary', textTransform: 'uppercase', fontWeight: 800, mb: 0.75 })}>
          Open Editors
        </Typography>
        <Box sx={(t) => ({
          height: 30,
          px: 1,
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          borderRadius: 1,
          bgcolor: 'background.paper',
          border: `1px solid ${t.palette.divider}`,
        })}>
          <Box sx={{ display: 'inline-flex', color: fileIcon(selectedPath).color }}>{fileIcon(selectedPath).icon}</Box>
          <Typography noWrap sx={{ flex: 1, minWidth: 0, fontSize: fontScale.s68, fontWeight: 600 }}>{selectedName}</Typography>
          <IconButton size="small" onClick={onCloseEditor} aria-label="Close open editor" sx={{ width: 20, height: 20, color: 'text.secondary' }}>
            <VscClose size={13} />
          </IconButton>
        </Box>
      </Box>
      <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto', p: 1 }}>
        <Stack direction="row" alignItems="center" spacing={0.5} sx={{ px: 0.5, mb: 0.6 }}>
          <VscChevronDown size={14} />
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s60, textTransform: 'uppercase', fontWeight: 800 })}>
            Northwind Checkout
          </Typography>
        </Stack>
        {tree.map((node) => (
          <TreeRow
            key={node.path}
            node={node}
            depth={0}
            selectedPath={selectedPath}
            openDirs={openDirs}
            onToggle={toggle}
            onSelect={onSelect}
          />
        ))}
      </Box>
      <Divider />
      <Stack direction="row" alignItems="center" spacing={0.5} sx={{ p: 1, borderBottom: 1, borderColor: 'divider' }}>
        <Tooltip title="Copy path">
          <IconButton size="small" onClick={onCopyPath} aria-label="Copy file path" sx={{ width: 26, height: 26, color: 'text.secondary' }}>
            <VscCopy size={14} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Download file">
          <IconButton size="small" onClick={onDownloadFile} aria-label="Download file" sx={{ width: 26, height: 26, color: 'text.secondary' }}>
            <VscCloudDownload size={14} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Download project ZIP">
          <IconButton size="small" onClick={onDownloadProject} aria-label="Download project ZIP" sx={{ width: 26, height: 26, color: 'text.secondary' }}>
            <VscFileZip size={14} />
          </IconButton>
        </Tooltip>
        <Box sx={{ flex: 1 }} />
        <Tooltip title="More">
          <IconButton size="small" aria-label="More editor actions" sx={{ width: 26, height: 26, color: 'text.secondary' }}>
            <VscEllipsis size={14} />
          </IconButton>
        </Tooltip>
      </Stack>
      {['Outline', 'Timeline'].map((label) => (
        <Stack key={label} direction="row" alignItems="center" spacing={0.75} sx={{ px: 1.5, height: 36, borderBottom: 1, borderColor: 'divider' }}>
          <VscChevronRight size={14} />
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s62, textTransform: 'uppercase', fontWeight: 800 })}>
            {label}
          </Typography>
        </Stack>
      ))}
    </Box>
  );
}

function IdeHeader({ projectName, fileCount, onDownloadProject }: { projectName: string; fileCount: number; onDownloadProject: () => void }) {
  return (
    <Stack
      data-testid="editor-header"
      direction="row"
      alignItems="center"
      justifyContent="space-between"
      sx={(t) => ({
        px: 2,
        height: 48,
        flexShrink: 0,
        borderBottom: `1px solid ${t.palette.divider}`,
        bgcolor: 'background.paper',
      })}
    >
      <Stack direction="row" alignItems="center" spacing={1.2} sx={{ minWidth: 0 }}>
        <LogoMark size={17} />
        <Typography sx={{ fontWeight: 800, fontSize: fontScale.s80 }} noWrap>Ironflyer IDE</Typography>
        <Typography sx={{ color: 'text.disabled' }}>/</Typography>
        <Typography sx={{ color: 'text.secondary', fontWeight: 600, fontSize: fontScale.s78 }} noWrap>{projectName}</Typography>
        <Tooltip title={`${fileCount} files in this Monaco workspace`}>
          <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: 'success.main', flexShrink: 0 }} />
        </Tooltip>
      </Stack>
      <Stack direction="row" alignItems="center" spacing={0.8}>
        <Tooltip title="Download project ZIP">
          <IconButton size="small" onClick={onDownloadProject} aria-label="Download full project ZIP" sx={{ width: 28, height: 28, color: 'text.secondary' }}>
            <VscFileZip size={15} />
          </IconButton>
        </Tooltip>
        <Chip
          size="small"
          label="PRO · CODE"
          sx={(t) => ({
            height: 25,
            borderRadius: 99,
            bgcolor: 'action.hover',
            color: 'primary.main',
            border: `1px solid ${t.palette.divider}`,
            fontFamily: t.brand.font.mono,
            fontSize: fontScale.s62,
            fontWeight: 800,
          })}
        />
      </Stack>
    </Stack>
  );
}

function EditorTabs({
  selectedPath,
  onCreateFile,
  onRenameFile,
  onDeleteFile,
  onCopyPath,
  onDownloadFile,
  onDownloadProject,
  onClose,
}: {
  selectedPath: string;
  onCreateFile: () => void;
  onRenameFile: () => void;
  onDeleteFile: () => void;
  onCopyPath: () => void;
  onDownloadFile: () => void;
  onDownloadProject: () => void;
  onClose: () => void;
}) {
  const icon = fileIcon(selectedPath);
  return (
    <Stack direction="row" alignItems="center" sx={(t) => ({ height: 44, borderBottom: `1px solid ${t.palette.divider}`, bgcolor: 'background.paper' })}>
      <Stack direction="row" alignItems="center" spacing={0.8} sx={(t) => ({
        alignSelf: 'stretch',
        px: 1.5,
        minWidth: 180,
        borderRight: `1px solid ${t.palette.divider}`,
        borderTop: `2px solid ${t.palette.primary.main}`,
        color: 'text.primary',
      })}>
        <Box sx={{ display: 'inline-flex', color: icon.color }}>{icon.icon}</Box>
        <Typography noWrap sx={{ flex: 1, minWidth: 0, fontSize: fontScale.s70, fontWeight: 650 }}>{selectedPath.split('/').pop()}</Typography>
        <Typography sx={{ fontSize: fontScale.s62, color: 'warning.main', fontWeight: 800 }}>M</Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close editor" sx={{ width: 20, height: 20, color: 'text.secondary' }}>
          <VscClose size={13} />
        </IconButton>
      </Stack>
      <Tooltip title="New file">
        <IconButton size="small" onClick={onCreateFile} aria-label="New editor file" sx={{ mx: 0.7, width: 28, height: 28, color: 'text.secondary' }}>
          <VscNewFile size={14} />
        </IconButton>
      </Tooltip>
      <Box sx={{ flex: 1 }} />
      <Stack direction="row" alignItems="center" spacing={1.2} sx={{ px: 1.4, color: 'text.secondary' }}>
        <Tooltip title="Rename file"><IconButton size="small" onClick={onRenameFile} aria-label="Rename current file" sx={{ width: 26, height: 26, color: 'text.secondary' }}><VscEdit size={14} /></IconButton></Tooltip>
        <Tooltip title="Delete file"><IconButton size="small" onClick={onDeleteFile} aria-label="Delete current file" sx={{ width: 26, height: 26, color: 'text.secondary' }}><VscTrash size={14} /></IconButton></Tooltip>
        <Tooltip title="Copy path"><IconButton size="small" onClick={onCopyPath} aria-label="Copy current file path" sx={{ width: 26, height: 26, color: 'text.secondary' }}><VscCopy size={14} /></IconButton></Tooltip>
        <Tooltip title="Download file"><IconButton size="small" onClick={onDownloadFile} aria-label="Download current file" sx={{ width: 26, height: 26, color: 'text.secondary' }}><VscCloudDownload size={14} /></IconButton></Tooltip>
        <Tooltip title="Download project ZIP"><IconButton size="small" onClick={onDownloadProject} aria-label="Download current project ZIP" sx={{ width: 26, height: 26, color: 'text.secondary' }}><VscFileZip size={14} /></IconButton></Tooltip>
        <VscRefresh size={14} />
        <VscEllipsis size={15} />
      </Stack>
    </Stack>
  );
}

function StatusBar({ selectedPath }: { selectedPath: string }) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      justifyContent="flex-end"
      spacing={2.5}
      sx={(t) => ({
        height: 34,
        px: 1.8,
        borderTop: `1px solid ${t.palette.divider}`,
        bgcolor: 'background.paper',
        color: 'text.secondary',
        fontFamily: t.brand.font.mono,
        fontSize: fontScale.s60,
      })}
    >
      <span>Ln 1, Col 1</span>
      <span>LF</span>
      <span>UTF-8</span>
      <span>Spaces: 2</span>
      <span>{languageForPath(selectedPath)}</span>
      <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: 'success.main' }} />
    </Stack>
  );
}

function PanelCard({ title, action, children }: { title: string; action?: string; children: React.ReactNode }) {
  return (
    <Box sx={(t) => ({
      border: `1px solid ${t.palette.divider}`,
      borderRadius: 2,
      bgcolor: 'background.paper',
      boxShadow: '0 18px 50px rgba(31, 41, 55, 0.06)',
      overflow: 'hidden',
    })}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1.7, pt: 1.5, pb: 1 }}>
        <Typography sx={{ fontSize: fontScale.s70, fontWeight: 800 }}>{title}</Typography>
        {action && <Typography sx={{ fontSize: fontScale.s64, color: 'primary.main', fontWeight: 700 }}>{action}</Typography>}
      </Stack>
      {children}
    </Box>
  );
}

function RightRail() {
  return (
    <Box sx={(t) => ({
      width: { xs: 240, xl: 285 },
      minWidth: { xs: 240, xl: 285 },
      borderLeft: `1px solid ${t.palette.divider}`,
      bgcolor: 'rgba(248, 250, 255, 0.45)',
      p: 1.5,
      overflow: 'auto',
    })}>
      <Stack spacing={1.5}>
        <PanelCard title="Active Agents">
          <Stack spacing={0.6} sx={{ px: 1.2, pb: 1.2 }}>
            {ACTIVE_AGENTS.map((agent) => (
              <Stack key={agent.name} direction="row" alignItems="center" spacing={1} sx={{ py: 0.8 }}>
                <Box sx={(t) => ({
                  width: 30,
                  height: 30,
                  borderRadius: 1.5,
                  display: 'grid',
                  placeItems: 'center',
                  bgcolor: 'action.hover',
                  color: `${agent.tone}.main`,
                  border: `1px solid ${t.palette.divider}`,
                })}>
                  {agent.tone === 'success' ? <VscPass size={15} /> : agent.tone === 'warning' ? <VscShield size={15} /> : <VscSymbolNamespace size={15} />}
                </Box>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography noWrap sx={{ fontSize: fontScale.s66, fontWeight: 750 }}>{agent.name}</Typography>
                  <Typography noWrap sx={{ fontSize: fontScale.s58, color: 'text.secondary' }}>{agent.status}</Typography>
                </Box>
                <Box sx={{ width: 26 }}>
                  <LinearProgress
                    variant="determinate"
                    value={agent.progress}
                    sx={{ height: 4, borderRadius: 99, bgcolor: 'action.hover' }}
                  />
                </Box>
              </Stack>
            ))}
            <Typography sx={{ pt: 0.4, color: 'primary.main', fontSize: fontScale.s64, fontWeight: 700 }}>View all agents</Typography>
          </Stack>
        </PanelCard>

        <PanelCard title="Gates & Status">
          <Stack direction="row" spacing={2} alignItems="center" sx={{ px: 1.7, pb: 1.3 }}>
            <Box sx={{
              width: 82,
              height: 82,
              borderRadius: '50%',
              background: 'conic-gradient(#704cff 0 42%, #38d4d4 42% 75%, #d9e3ff 75% 100%)',
              display: 'grid',
              placeItems: 'center',
              flexShrink: 0,
            }}>
              <Box sx={{ width: 50, height: 50, borderRadius: '50%', bgcolor: 'background.paper' }} />
            </Box>
            <Box>
              <Typography sx={{ fontSize: fontScale.s64, color: 'text.secondary', fontWeight: 700 }}>All gates</Typography>
              <Typography sx={{ fontSize: 26, fontWeight: 850, lineHeight: 1.05 }}>5 / 5</Typography>
              <Typography sx={{ fontSize: fontScale.s66, color: 'primary.main', fontWeight: 700 }}>Open</Typography>
            </Box>
          </Stack>
          <Stack spacing={0.85} sx={{ px: 1.7, pb: 1.4 }}>
            {GATES.map((gate) => (
              <Stack key={gate.name} direction="row" alignItems="center" spacing={1}>
                <Box sx={{ color: `${gate.tone}.main`, display: 'inline-flex' }}><VscPass size={14} /></Box>
                <Typography sx={{ flex: 1, fontSize: fontScale.s66, fontWeight: 650 }}>{gate.name}</Typography>
                <Typography sx={{ fontSize: fontScale.s62, color: `${gate.tone}.main`, fontWeight: 750 }}>{gate.status}</Typography>
              </Stack>
            ))}
          </Stack>
        </PanelCard>

        <PanelCard title="Recent Activity" action="View all">
          <Stack spacing={1} sx={{ px: 1.7, pb: 1.5 }}>
            {ACTIVITY.map((item) => (
              <Stack key={item.text} direction="row" alignItems="center" spacing={1}>
                <Box sx={{ color: `${item.tone}.main`, display: 'inline-flex' }}>{item.tone === 'success' ? <VscPass size={14} /> : <VscFile size={14} />}</Box>
                <Typography noWrap sx={{ flex: 1, minWidth: 0, fontSize: fontScale.s62, color: 'text.secondary', fontWeight: 600 }}>{item.text}</Typography>
                <Typography sx={{ fontSize: fontScale.s58, color: 'text.disabled' }}>{item.time}</Typography>
              </Stack>
            ))}
          </Stack>
        </PanelCard>
      </Stack>
    </Box>
  );
}

const beforeMount: BeforeMount = (monaco) => {
  monaco.editor.defineTheme('ironflyer-light', {
    base: 'vs',
    inherit: true,
    rules: [
      { token: 'keyword', foreground: '0047ff', fontStyle: 'bold' },
      { token: 'string', foreground: '057a55' },
      { token: 'number', foreground: '7c3aed' },
      { token: 'comment', foreground: '7b8794' },
    ],
    colors: {
      'editor.background': '#ffffff',
      'editor.foreground': '#18244b',
      'editorLineNumber.foreground': '#7f8eb5',
      'editorLineNumber.activeForeground': '#0047ff',
      'editor.selectionBackground': '#dce7ff',
      'editorCursor.foreground': '#0047ff',
      'editorGutter.background': '#ffffff',
    },
  });
};

export function CodePane() {
  const project = useStudio((s) => s.current);
  const generatedFiles = useStudio((s) => s.generatedFiles);
  const writeGeneratedFiles = useStudio((s) => s.writeGeneratedFiles);
  const clearGeneratedFiles = useStudio((s) => s.clearGeneratedFiles);
  const files = generatedFiles.length > 0 ? generatedFiles : SAMPLE_FILES;
  const [selectedPath, setSelectedPath] = useState(() => files[0]?.path ?? 'README.md');
  const [dialog, setDialog] = useState<EditorDialogState>(null);
  const selectedFile = files.find((file) => file.path === selectedPath) ?? files[0] ?? SAMPLE_FILES[0]!;

  const replaceFiles = (nextFiles: { path: string; content: string }[]) => {
    clearGeneratedFiles();
    if (nextFiles.length > 0) writeGeneratedFiles(nextFiles);
  };

  const handleChange = (value?: string) => {
    if (typeof value !== 'string') return;
    if (generatedFiles.length === 0) {
      replaceFiles(files.map((file) => file.path === selectedFile.path ? { path: file.path, content: value } : { path: file.path, content: file.content }));
      return;
    }
    writeGeneratedFiles([{ path: selectedFile.path, content: value }]);
  };

  const selectFile = (path: string) => {
    setSelectedPath(path);
  };

  const submitFileDialog = (path: string) => {
    if (dialog?.kind === 'create') {
      replaceFiles([...files.map((file) => ({ path: file.path, content: file.content })), { path, content: starterContent(path) }]);
      setSelectedPath(path);
      setDialog(null);
      toast(`Created ${path}.`, 'success');
      return;
    }

    if (dialog?.kind === 'rename') {
      const nextFiles = files.map((file) => (
        file.path === selectedFile.path ? { path, content: file.content } : { path: file.path, content: file.content }
      ));
      replaceFiles(nextFiles);
      setSelectedPath(path);
      setDialog(null);
      toast(`Renamed to ${path}.`, 'success');
    }
  };

  const closeEditor = () => {
    const currentIndex = files.findIndex((file) => file.path === selectedFile.path);
    const fallback = files[currentIndex + 1] ?? files[currentIndex - 1] ?? files[0];
    if (fallback) setSelectedPath(fallback.path);
  };

  const deleteSelectedFile = () => {
    const nextFiles = files
      .filter((file) => file.path !== selectedFile.path)
      .map((file) => ({ path: file.path, content: file.content }));
    replaceFiles(nextFiles);
    setSelectedPath(nextFiles[0]?.path ?? 'README.md');
    setDialog(null);
    toast(`Deleted ${selectedFile.path}.`, 'success');
  };

  const copyPath = () => {
    void navigator.clipboard?.writeText(selectedFile.path);
    toast('Path copied.', 'success');
  };

  const downloadFile = () => {
    downloadText(selectedFile.path, selectedFile.content);
    toast('File downloaded.', 'success');
  };

  const downloadZip = () => {
    if (files.length === 0) {
      toast('No project files to download.', 'info');
      return;
    }
    downloadProjectZip(project.name, files);
    toast('Project ZIP downloaded.', 'success');
  };

  return (
    <Box sx={{
      flex: 1,
      minWidth: 0,
      height: '100%',
      p: 0,
      bgcolor: 'background.default',
      overflow: 'hidden',
    }}>
      <Box data-testid="code-pane-surface" sx={(t) => ({
        height: '100%',
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        border: `1px solid ${t.palette.divider}`,
        borderRadius: 0,
        bgcolor: 'background.paper',
        overflow: 'hidden',
      })}>
        <IdeHeader projectName={project.name} fileCount={files.length} onDownloadProject={downloadZip} />
        <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Explorer
            files={files}
            selectedPath={selectedFile.path}
            onSelect={selectFile}
            onCreateFile={() => setDialog({ kind: 'create' })}
            onRenameFile={() => setDialog({ kind: 'rename' })}
            onDeleteFile={() => setDialog({ kind: 'delete' })}
            onCopyPath={copyPath}
            onDownloadFile={downloadFile}
            onDownloadProject={downloadZip}
            onCloseEditor={closeEditor}
          />
          <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', bgcolor: 'background.paper' }}>
            <EditorTabs
              selectedPath={selectedFile.path}
              onCreateFile={() => setDialog({ kind: 'create' })}
              onRenameFile={() => setDialog({ kind: 'rename' })}
              onDeleteFile={() => setDialog({ kind: 'delete' })}
              onCopyPath={copyPath}
              onDownloadFile={downloadFile}
              onDownloadProject={downloadZip}
              onClose={closeEditor}
            />
            <Box sx={{ flex: 1, minHeight: 0 }}>
              <Editor
                path={selectedFile.path}
                value={selectedFile.content}
                language={languageForPath(selectedFile.path)}
                theme="ironflyer-light"
                beforeMount={beforeMount}
                onChange={handleChange}
                options={{
                  automaticLayout: true,
                  bracketPairColorization: { enabled: true },
                  contextmenu: true,
                  cursorBlinking: 'smooth',
                  dragAndDrop: true,
                  find: { addExtraSpaceOnTop: false, autoFindInSelection: 'never' },
                  folding: true,
                  foldingHighlight: true,
                  formatOnPaste: true,
                  formatOnType: true,
                  fontFamily: "'Geist Mono', 'SFMono-Regular', Menlo, Monaco, Consolas, monospace",
                  fontSize: 13,
                  glyphMargin: true,
                  guides: { bracketPairs: true, indentation: true },
                  lineHeight: 24,
                  minimap: { enabled: false },
                  padding: { top: 18, bottom: 18 },
                  quickSuggestions: true,
                  scrollBeyondLastLine: false,
                  scrollbar: { horizontalScrollbarSize: 10, verticalScrollbarSize: 10 },
                  smoothScrolling: true,
                  stickyScroll: { enabled: true },
                  suggest: { showIcons: true, snippetsPreventQuickSuggestions: false },
                  wordWrap: 'on',
                  tabSize: 2,
                  renderLineHighlight: 'line',
                  overviewRulerBorder: false,
                  hideCursorInOverviewRuler: true,
                }}
              />
            </Box>
            <StatusBar selectedPath={selectedFile.path} />
          </Box>
          <RightRail />
        </Box>
      </Box>
      <EditorFileDialog
        open={dialog?.kind === 'create' || dialog?.kind === 'rename'}
        mode={dialog?.kind === 'rename' ? 'rename' : 'create'}
        initialPath={dialog?.kind === 'rename' ? selectedFile.path : 'src/new-file.ts'}
        existingPaths={files.map((file) => file.path)}
        onClose={() => setDialog(null)}
        onSubmit={submitFileDialog}
      />
      <EditorConfirmDialog
        open={dialog?.kind === 'delete'}
        title="Delete file?"
        text={`This removes ${selectedFile.path} from the current workspace file set.`}
        confirmText="Delete"
        danger
        onClose={() => setDialog(null)}
        onConfirm={deleteSelectedFile}
      />
    </Box>
  );
}
