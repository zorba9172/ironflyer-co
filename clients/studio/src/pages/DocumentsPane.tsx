import { useMemo, useRef, useState } from 'react';
import { Box, Button, Card, Chip, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption, CodeEditor, toast, Lightbox } from '@ironflyer/ui-web/fx';
import { useThemeMode } from '@ironflyer/ui-web';
import { useStudio, type Attachment } from '../store';
import { DocDialog } from '../components/DocDialog';
import { text } from '@ironflyer/design-tokens/brand';

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

const FileGlyph = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8zM14 2v6h6" /></svg>
);
const PlusGlyph = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M12 5v14M5 12h14" /></svg>
);
const UploadGlyph = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12" /></svg>
);

// One place to bring research into the build: create a doc inline or upload
// files. Text docs ground the chat; images attach as visual references.
export function DocumentsPane() {
  const t = useTheme();
  const { mode } = useThemeMode();
  const attachments = useStudio((s) => s.attachments);
  const addAttachments = useStudio((s) => s.addAttachments);
  const removeAttachment = useStudio((s) => s.removeAttachment);
  const updateAttachment = useStudio((s) => s.updateAttachment);

  const [view, setView] = useState<'tiles' | 'table'>('tiles');
  const [dragging, setDragging] = useState(false);
  const [openDoc, setOpenDoc] = useState<Attachment | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('untitled.md');
  const [newText, setNewText] = useState('');
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

  const columns = useMemo<DataGridColumn<Attachment>[]>(() => [
    { field: 'name', headerName: 'Name', flex: 1, minWidth: 220 },
    { field: 'kind', headerName: 'Type', width: 110, cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? <Chip size="small" label={data.kind} sx={{ height: 20, fontSize: text.s64, bgcolor: 'action.hover' }} /> : null },
    { colId: 'grounds', headerName: 'Grounds chat', width: 130, cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? <Typography sx={{ fontSize: text.s80, color: data.text ? 'success.main' : 'text.disabled' }}>{data.text ? '● yes' : '— no'}</Typography> : null },
    { field: 'size', headerName: 'Size', width: 100, valueFormatter: ({ value }) => fmtSize(Number(value)) },
    { colId: 'remove', headerName: '', width: 70, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<Attachment>) => data ? <Button size="small" color="inherit" onClick={(e) => { e.stopPropagation(); removeAttachment(data.id); }}>Remove</Button> : null },
  ], [removeAttachment]);

  const grounding = attachments.filter((a) => a.text).length;

  // Headline visual — mirrors the document set by type. The center names how
  // many docs ground the chat (what the AI actually reads end-to-end), so the
  // operator reads the corpus in one glance before the table/tiles below.
  const typeDonut = useMemo<EChartsOption>(() => {
    const byKind: Record<string, number> = {};
    for (const a of attachments) byKind[a.kind] = (byKind[a.kind] ?? 0) + 1;
    const tone: Record<string, string> = {
      text: t.palette.success.main,
      image: t.brand.accent.secondary,
      file: t.palette.text.disabled,
    };
    const data = Object.entries(byKind).map(([kind, value]) => ({
      value, name: kind, itemStyle: { color: tone[kind] ?? t.palette.primary.main },
    }));
    return {
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
      series: [{
        type: 'pie', radius: ['58%', '80%'], avoidLabelOverlap: true,
        itemStyle: { borderColor: t.palette.background.paper, borderWidth: 2 },
        label: { show: true, position: 'center', formatter: grounding > 0 ? `${grounding}\ngrounding` : `${attachments.length}\ndocs`, color: grounding > 0 ? t.palette.success.main : t.palette.text.secondary, fontSize: 22, lineHeight: 22 },
        data,
      }],
    };
  }, [attachments, grounding, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1040, mx: 'auto' }}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ mb: 2.5, gap: 2 }}>
          <Box>
            <Typography variant="h4" sx={{ fontSize: text.s160, mb: 0.5 }}>Documents</Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Everything the build reads, in one place. Create a doc or upload files — text docs ground the chat and every agent.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
            <Button variant="outlined" color="inherit" startIcon={<UploadGlyph />} onClick={() => inputRef.current?.click()}>Upload</Button>
            <Button variant="contained" startIcon={<PlusGlyph />} onClick={() => setCreating(true)}>New document</Button>
          </Stack>
        </Stack>

        {attachments.length > 0 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
            <Card sx={{ p: 2 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Documents by type</Typography>
              <Chart option={typeDonut} height={200} />
            </Card>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5 }}>
              {[
                { label: 'Documents', value: String(attachments.length), sub: 'in this project' },
                { label: 'Grounding chat', value: String(grounding), sub: 'read by every agent' },
                { label: 'Images', value: String(attachments.filter((a) => a.kind === 'image').length), sub: 'visual references' },
                { label: 'Other files', value: String(attachments.filter((a) => a.kind === 'file').length), sub: 'attached, not parsed' },
              ].map((m) => (
                <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                  <Typography variant="h4" sx={{ fontSize: text.s180, mt: 0.5 }}>{m.value}</Typography>
                  <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
                </Card>
              ))}
            </Box>
          </Box>
        )}

        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
          <Typography sx={{ color: 'text.secondary', fontSize: text.s90 }}>
            {attachments.length} document{attachments.length === 1 ? '' : 's'}{grounding > 0 ? ` · ${grounding} grounding the chat` : ''}
          </Typography>
          <ToggleButtonGroup exclusive size="small" value={view} onChange={(_, v) => v && setView(v)} sx={{ '& .MuiToggleButton-root': { px: 1.25, py: 0.5, textTransform: 'none', borderColor: 'divider' } }}>
            <ToggleButton value="tiles">Tiles</ToggleButton>
            <ToggleButton value="table">Table</ToggleButton>
          </ToggleButtonGroup>
        </Stack>

        <input ref={inputRef} type="file" multiple hidden accept=".md,.markdown,.txt,.json,.csv,.yml,.yaml,.html,image/*" onChange={(e) => { void ingest(e.target.files); e.target.value = ''; }} />

        <Box
          onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => { e.preventDefault(); setDragging(false); void ingest(e.dataTransfer.files); }}
          sx={{ borderRadius: 3, border: '1.5px dashed', borderColor: dragging ? 'primary.main' : 'transparent', bgcolor: dragging ? 'action.hover' : 'transparent', transition: (t) => `all ${t.brand.motion.fast}`, p: dragging ? 1 : 0 }}
        >
          {attachments.length === 0 ? (
            <Card
              onClick={() => inputRef.current?.click()}
              sx={{ p: 6, textAlign: 'center', cursor: 'pointer', border: '1.5px dashed', borderColor: 'divider', '&:hover': { borderColor: 'text.disabled' } }}
            >
              <Box sx={{ color: 'text.disabled', mb: 1.5, display: 'flex', justifyContent: 'center' }}><UploadGlyph /></Box>
              <Typography sx={{ fontWeight: 600, mb: 0.5 }}>Drop files here, or use New document</Typography>
              <Typography sx={{ fontSize: text.s85, color: 'text.disabled' }}>md, txt, json, csv, yaml, html, png, jpg — text files ground the chat.</Typography>
            </Card>
          ) : view === 'tiles' ? (
            <Lightbox>
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 1.5 }}>
                {attachments.map((a) => (
                  <Card key={a.id} onClick={a.kind === 'image' ? undefined : () => setOpenDoc(a)} sx={{ p: 0, overflow: 'hidden', position: 'relative', cursor: a.kind === 'image' ? 'default' : 'pointer', '&:hover': { borderColor: 'text.disabled' } }}>
                    <IconButton size="small" aria-label="Remove" onClick={(e) => { e.stopPropagation(); removeAttachment(a.id); }} sx={{ position: 'absolute', top: 6, right: 6, zIndex: 1, bgcolor: 'background.paper', border: 1, borderColor: 'divider', color: 'text.disabled', width: 24, height: 24 }}>✕</IconButton>
                    {a.kind === 'image' && a.dataUrl ? (
                      <Box component="a" href={a.dataUrl} data-fancybox="docs" data-caption={a.name} sx={{ display: 'block', cursor: 'zoom-in' }}>
                        <Box component="img" src={a.dataUrl} alt={a.name} sx={{ width: '100%', height: 120, objectFit: 'cover', display: 'block' }} />
                      </Box>
                    ) : (
                      <Box sx={(t) => ({ height: 120, display: 'grid', placeItems: 'center', color: 'text.disabled', backgroundImage: t.brand.gradient.signatureSoft })}><FileGlyph /></Box>
                    )}
                    <Box sx={{ p: 1.5 }}>
                      <Typography sx={{ fontSize: text.s84, fontWeight: 600 }} noWrap>{a.name}</Typography>
                      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })}>{a.kind}{a.text ? ' · grounds chat' : ''} · {fmtSize(a.size)}</Typography>
                    </Box>
                  </Card>
                ))}
              </Box>
            </Lightbox>
          ) : (
            <DataGrid
              rows={attachments}
              columns={columns}
              getRowId={(row) => row.id}
              density="compact"
              emptyLabel="No documents."
              height={attachments.length > 8 ? 460 : 300}
              minHeight={220}
              pagination={attachments.length > 10}
              pageSize={10}
              onRowClick={(row) => setOpenDoc(row)}
            />
          )}
        </Box>

        <DocDialog attachment={openDoc} onClose={() => setOpenDoc(null)} onSave={updateAttachment} />

        <Dialog open={creating} onClose={() => setCreating(false)} maxWidth="md" fullWidth slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
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
