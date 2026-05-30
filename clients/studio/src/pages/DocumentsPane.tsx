import { useMemo, useRef, useState } from 'react';
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { CodeEditor, toast, Lightbox } from '@ironflyer/ui-web/fx';
import { useThemeMode } from '../theme';
import { useStudio, type Attachment } from '../store';
import { DocDialog } from '../components/DocDialog';
import { StudioChart, donutOption, type EChartsOption } from '../components/charts';
import { StudioDataGrid, StudioTableShell, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { text } from '@ironflyer/design-tokens/brand';
import { GlassPanel, SectionHeader, StatCard } from '../components/studio';

function readFile(file: File): Promise<Attachment> {
  const id = `${file.name}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  const isImage = file.type.startsWith('image/');
  const isText = file.type.startsWith('text/') || /\.(md|markdown|txt|json|csv|ya?ml|html?)$/i.test(file.name);
  return new Promise((resolve) => {
    if (!isImage && !isText) { resolve({ id, name: file.name, size: file.size, kind: 'file' }); return; }
    const r = new FileReader();
    r.onload = () => resolve({
      id, name: file.name, size: file.size,
      kind: isImage ? 'image' : 'text',
      text: isText ? String(r.result) : undefined,
      dataUrl: isImage ? String(r.result) : undefined,
    });
    r.onerror = () => resolve({ id, name: file.name, size: file.size, kind: 'file' });
    if (isImage) r.readAsDataURL(file); else r.readAsText(file);
  });
}

const fmtSize = (n: number) => (n < 1024 ? `${n} B` : n < 1048576 ? `${(n / 1024).toFixed(0)} KB` : `${(n / 1048576).toFixed(1)} MB`);

// --- Inline glyphs (no external lib) ----------------------------------------

function FileGlyph({ size = 32 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
      <path d="M14 2v6h6" />
      <path d="M10 13h6M10 17h4" />
    </svg>
  );
}

function ImageGlyph({ size = 32 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="18" height="18" rx="3" />
      <circle cx="8.5" cy="8.5" r="1.5" />
      <path d="M21 15l-5-5L5 21" />
    </svg>
  );
}

function PlusGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function UploadGlyph() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12" />
    </svg>
  );
}

function RemoveGlyph() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  );
}

// An empty-state drop zone — large, calm, action-ready.
function EmptyDropZone({ onClick }: { onClick: () => void }) {
  return (
    <GlassPanel
      interactive
      pad={6}
      sx={{ textAlign: 'center', border: '1.5px dashed', borderColor: 'divider', cursor: 'pointer' }}
      onClick={onClick}
    >
      <Box sx={{ color: 'text.disabled', mb: 2, display: 'flex', justifyContent: 'center' }}>
        <UploadGlyph />
      </Box>
      <Typography sx={{ fontWeight: 700, fontSize: text.s95, mb: 0.75 }}>
        Drop files here, or click to upload
      </Typography>
      <Typography sx={{ fontSize: text.s84, color: 'text.disabled' }}>
        md, txt, json, csv, yaml, html, png, jpg — text files ground the chat.
      </Typography>
    </GlassPanel>
  );
}

// A document tile card — GlassPanel with kind glyph, name, metadata.
function DocTile({ attachment, onOpen, onRemove }: { attachment: Attachment; onOpen: () => void; onRemove: () => void }) {
  const t = useTheme();
  const isImage = attachment.kind === 'image';
  const isText = attachment.kind === 'text';
  const glyphColor = isText ? t.palette.success.main : isImage ? t.palette.primary.main : t.palette.text.disabled;

  return (
    <GlassPanel
      interactive={!isImage}
      pad={0}
      sx={{ overflow: 'hidden', position: 'relative' }}
      onClick={isImage ? undefined : onOpen}
    >
      <IconButton
        size="small"
        aria-label="Remove document"
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        sx={{
          position: 'absolute', top: 6, right: 6, zIndex: 1,
          bgcolor: 'background.paper', border: '1px solid', borderColor: 'divider',
          color: 'text.disabled', width: 22, height: 22,
          '&:hover': { color: 'error.main', borderColor: 'error.main' },
        }}
      >
        <RemoveGlyph />
      </IconButton>

      {/* Thumbnail / icon area */}
      {isImage && attachment.dataUrl ? (
        <Box component="a" href={attachment.dataUrl} data-fancybox="docs" data-caption={attachment.name} sx={{ display: 'block', cursor: 'zoom-in' }}>
          <Box component="img" src={attachment.dataUrl} alt={attachment.name} sx={{ width: '100%', height: 110, objectFit: 'cover', display: 'block' }} />
        </Box>
      ) : (
        <Box sx={(_th) => ({
          height: 110, display: 'grid', placeItems: 'center',
          background: `radial-gradient(120% 120% at 30% 20%, ${glyphColor}18, ${glyphColor}06 70%)`,
          color: glyphColor,
          borderBottom: '1px solid', borderColor: 'borderSubtle',
        })}>
          {isImage ? <ImageGlyph /> : <FileGlyph />}
        </Box>
      )}

      {/* Meta */}
      <Box sx={{ p: 1.5 }}>
        <Typography sx={{ fontSize: text.s86, fontWeight: 700, mb: 0.25 }} noWrap>{attachment.name}</Typography>
        <Stack direction="row" alignItems="center" spacing={0.75}>
          <Chip size="small" label={attachment.kind} sx={{ height: 16, fontSize: text.s58, bgcolor: 'action.hover', color: 'text.secondary' }} />
          {attachment.text && (
            <Chip size="small" label="grounds chat" sx={(th) => ({ height: 16, fontSize: text.s58, bgcolor: `${th.palette.success.main}18`, color: 'success.main' })} />
          )}
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, color: 'text.disabled', ml: 'auto !important' })}>
            {fmtSize(attachment.size)}
          </Typography>
        </Stack>
      </Box>
    </GlassPanel>
  );
}

// One place to bring research into the build: create a doc inline or upload
// files. Text docs ground the chat; images attach as visual references.
// The donut mirrors the corpus by type so the operator reads the document set
// in one glance before scanning the grid/tile list below.
export function DocumentsPane() {
  const t = useTheme();
  const { mode } = useThemeMode();
  const attachments = useStudio((s) => s.attachments);
  const addAttachments = useStudio((s) => s.addAttachments);
  const removeAttachment = useStudio((s) => s.removeAttachment);
  const updateAttachment = useStudio((s) => s.updateAttachment);

  const [view, setView] = useState<'tiles' | 'table'>('tiles');
  const [docTab, setDocTab] = useState('all');
  const [dragging, setDragging] = useState(false);
  const [openDoc, setOpenDoc] = useState<Attachment | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('untitled.md');
  const [newText, setNewText] = useState('');
  const [search, setSearch] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const ingest = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const items = await Promise.all(Array.from(files).map(readFile));
    addAttachments(items);
    toast(`Added ${items.length} item${items.length > 1 ? 's' : ''} to project research.`, 'success');
  };

  const createDoc = () => {
    const name = newName.trim() || 'untitled.md';
    addAttachments([{
      id: `${name}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      name, kind: 'text', size: new Blob([newText]).size, text: newText,
    }]);
    toast(`Created ${name} — it grounds the chat.`, 'success');
    setCreating(false);
    setNewName('untitled.md');
    setNewText('');
  };

  const grounding = attachments.filter((a) => a.text).length;
  const images = attachments.filter((a) => a.kind === 'image').length;
  const other = attachments.filter((a) => a.kind === 'file').length;

  const docTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: attachments.length },
    { value: 'text', label: 'Text', count: grounding, tone: grounding ? 'success' : 'default' },
    { value: 'image', label: 'Images', count: images, tone: 'info' },
    { value: 'file', label: 'Files', count: other },
    { value: 'grounding', label: 'Grounding', count: grounding, tone: grounding ? 'success' : 'default' },
  ], [attachments.length, grounding, images, other]);

  // Tab + search filtered view for tiles/table.
  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return attachments.filter((a) => {
      if (docTab === 'text' && a.kind !== 'text') return false;
      if (docTab === 'image' && a.kind !== 'image') return false;
      if (docTab === 'file' && a.kind !== 'file') return false;
      if (docTab === 'grounding' && !a.text) return false;
      return !q || a.name.toLowerCase().includes(q) || a.kind.includes(q);
    });
  }, [attachments, docTab, search]);

  // Headline visual — corpus by type, center label = how many ground the chat.
  const typeDonut = useMemo<EChartsOption>(() => {
    const byKind: Record<string, number> = {};
    for (const a of attachments) byKind[a.kind] = (byKind[a.kind] ?? 0) + 1;
    const tone: Record<string, string> = {
      text: t.palette.success.main,
      image: t.palette.primary.main,
      file: t.palette.text.disabled,
    };
    const data = Object.entries(byKind).map(([kind, value]) => ({
      value, name: kind, color: tone[kind] ?? t.palette.primary.main,
    }));
    return donutOption(t, {
      data,
      centerLabel: grounding > 0 ? `${grounding}\ngrounding` : `${attachments.length}\ndocs`,
      centerColor: grounding > 0 ? t.palette.success.main : t.palette.text.secondary,
    });
  }, [attachments, grounding, t]);

  const columns = useMemo<DataGridColumn<Attachment>[]>(() => [
    {
      field: 'name', headerName: 'Name', flex: 1, minWidth: 220,
      cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? (
        <Stack direction="row" alignItems="center" spacing={0.75}>
          <Box sx={{ color: data.kind === 'text' ? 'success.main' : data.kind === 'image' ? 'info.main' : 'text.disabled', display: 'inline-flex' }}>
            {data.kind === 'image' ? <ImageGlyph size={14} /> : <FileGlyph size={14} />}
          </Box>
          <Typography sx={{ fontSize: text.s86 }} noWrap>{data.name}</Typography>
        </Stack>
      ) : null,
    },
    {
      field: 'kind', headerName: 'Type', width: 110,
      cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? (
        <Chip size="small" label={data.kind} sx={{ height: 20, fontSize: text.s64, bgcolor: 'action.hover' }} />
      ) : null,
    },
    {
      colId: 'grounds', headerName: 'Grounds chat', width: 130,
      cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? (
        <Typography sx={{ fontSize: text.s80, color: data.text ? 'success.main' : 'text.disabled' }}>
          {data.text ? '● yes' : '— no'}
        </Typography>
      ) : null,
    },
    { field: 'size', headerName: 'Size', width: 100, valueFormatter: ({ value }) => fmtSize(Number(value)) },
    {
      colId: 'remove', headerName: '', width: 70, sortable: false, filter: false,
      cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? (
        <Button size="small" color="inherit" onClick={(e) => { e.stopPropagation(); removeAttachment(data.id); }}>Remove</Button>
      ) : null,
    },
  ], [removeAttachment]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: { xs: 2, md: 3 } }}>
      <Box sx={{ maxWidth: 1160, mx: 'auto' }}>

        {/* ── Header ─────────────────────────────────────────────────────── */}
        <SectionHeader
          eyebrow="Project knowledge"
          title="Documents"
          subtitle="Everything the build reads, in one place. Create a doc or upload files — text docs ground the chat and every agent."
          actions={
            <Stack direction="row" spacing={1}>
              <Button variant="outlined" color="inherit" startIcon={<UploadGlyph />} onClick={() => inputRef.current?.click()}>
                Upload
              </Button>
              <Button variant="contained" startIcon={<PlusGlyph />} onClick={() => setCreating(true)}>
                New document
              </Button>
            </Stack>
          }
        />

        {/* ── Stats + donut (only when docs exist) ───────────────────────── */}
        {attachments.length > 0 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '220px 1fr' }, gap: 2, mb: 3, alignItems: 'start' }}>
            {/* Corpus donut */}
            <GlassPanel pad={2.5}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
                Corpus by type
              </Typography>
              <StudioChart option={typeDonut} height={190} />
            </GlassPanel>

            {/* KPI strip */}
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(2, 1fr)' }, gap: 1.5 }}>
              <StatCard label="Documents" value={attachments.length} hint="in this project" accent={t.palette.primary.main} />
              <StatCard label="Grounding chat" value={grounding} hint="read by every agent" accent={t.palette.success.main} />
              <StatCard label="Images" value={images} hint="visual references" accent={t.palette.primary.main} />
              <StatCard label="Other files" value={other} hint="attached, not parsed" accent={t.palette.text.disabled} />
            </Box>
          </Box>
        )}

        {/* ── Hidden file input ───────────────────────────────────────────── */}
        <input ref={inputRef} type="file" multiple hidden accept=".md,.markdown,.txt,.json,.csv,.yml,.yaml,.html,image/*" onChange={(e) => { void ingest(e.target.files); e.target.value = ''; }} />

        {/* ── Drop zone + content ─────────────────────────────────────────── */}
        <StudioTableShell
          title="Document library"
          subtitle={`${visible.length.toLocaleString()} visible · ${attachments.length.toLocaleString()} total · ${grounding.toLocaleString()} grounding chat`}
          tabs={docTabs}
          activeTab={docTab}
          onTabChange={setDocTab}
          searchValue={search}
          onSearchChange={setSearch}
          searchPlaceholder="Search documents"
          actions={
            <ToggleButtonGroup
              exclusive size="small" value={view} onChange={(_, v) => v && setView(v)}
              sx={{ '& .MuiToggleButton-root': { px: 1.5, py: 0.5, textTransform: 'none', borderColor: 'divider', fontSize: text.s82 } }}
            >
              <ToggleButton value="tiles">Tiles</ToggleButton>
              <ToggleButton value="table">Table</ToggleButton>
            </ToggleButtonGroup>
          }
          footer="Text documents ground the chat; images stay available as visual references; files remain attached to the project."
        >
          <Box
            onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => { e.preventDefault(); setDragging(false); void ingest(e.dataTransfer.files); }}
            sx={{
              borderRadius: (th) => `${th.studio.radius.sm}px`,
              border: '1.5px dashed',
              borderColor: dragging ? 'primary.main' : 'transparent',
              bgcolor: dragging ? 'action.hover' : 'transparent',
              transition: (th) => `all ${th.studio.motion.fast}`,
              p: dragging ? 1 : view === 'tiles' ? 2 : 0,
            }}
          >
            {attachments.length === 0 ? (
              <EmptyDropZone onClick={() => inputRef.current?.click()} />
            ) : view === 'tiles' ? (
              <Lightbox>
                <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 1.5 }}>
                  {visible.map((a) => (
                    <DocTile
                      key={a.id}
                      attachment={a}
                      onOpen={() => setOpenDoc(a)}
                      onRemove={() => removeAttachment(a.id)}
                    />
                  ))}
                  {visible.length === 0 && (
                    <Box sx={{ gridColumn: '1 / -1', py: 4, textAlign: 'center' }}>
                      <Typography sx={{ color: 'text.disabled' }}>No documents match "{search}".</Typography>
                    </Box>
                  )}
                </Box>
              </Lightbox>
            ) : (
              <StudioDataGrid
                rows={visible}
                columns={columns}
                getRowId={(row) => row.id}
                density="compact"
                emptyLabel={search ? `No documents match "${search}".` : 'No documents.'}
                height={attachments.length > 8 ? 460 : 300}
                minHeight={220}
                pagination={attachments.length > 10}
                pageSize={10}
                onRowClick={(row) => setOpenDoc(row)}
              />
            )}
          </Box>
        </StudioTableShell>

        {/* ── Docs dialog (view/edit) ────────────────────────────────────── */}
        <DocDialog attachment={openDoc} onClose={() => setOpenDoc(null)} onSave={updateAttachment} />

        {/* ── New document dialog ────────────────────────────────────────── */}
        <Dialog open={creating} onClose={() => setCreating(false)} maxWidth="md" fullWidth
          slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
          <DialogTitle sx={{ fontWeight: 700, fontSize: text.s105 }}>New document</DialogTitle>
          <DialogContent dividers sx={{ p: 0 }}>
            <Box sx={{ p: 2.5, pb: 1.5 }}>
              <TextField label="File name" value={newName} onChange={(e) => setNewName(e.target.value)} fullWidth size="small" placeholder="spec.md" />
            </Box>
            <Box sx={{ px: 2.5, pb: 2.5, '& .cm-editor': { maxHeight: 420, borderRadius: 8 } }}>
              <CodeEditor value={newText} language={/\.json$/i.test(newName) ? 'json' : undefined} dark={mode === 'dark'} height={420} onChange={setNewText} />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button color="inherit" onClick={() => setCreating(false)}>Cancel</Button>
            <Button variant="contained" disabled={!newName.trim()} onClick={createDoc}>Create document</Button>
          </DialogActions>
        </Dialog>

      </Box>
    </Box>
  );
}
