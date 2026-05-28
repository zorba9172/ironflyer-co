import { useRef, useState } from 'react';
import { Box, Button, Card, Chip, IconButton, Stack, TextField, Typography } from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';
import { useStudio, type Attachment } from '../store';
import { LogoMark } from '../components/LogoMark';

const examples = [
  'Ship a B2B SaaS with team workspaces and Stripe billing.',
  'No data leaves the EU; every endpoint requires auth.',
  'Mobile-first; must pass Lighthouse ≥ 90.',
];

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

export function GoalsPane() {
  const { constitution, setConstitution, attachments, addAttachments, removeAttachment } = useStudio();
  const [draft, setDraft] = useState(constitution);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const dirty = draft.trim() !== constitution.trim();

  const ingest = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const items = await Promise.all(Array.from(files).map(readFile));
    addAttachments(items);
    toast(`Added ${items.length} item${items.length > 1 ? 's' : ''} to project research.`, 'success');
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: { xs: 3, md: 4 } }}>
      <Box sx={{ maxWidth: 1040, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 4 }}>
          <Box sx={(t) => ({ width: 52, height: 52, borderRadius: 3, display: 'grid', placeItems: 'center', border: 1, borderColor: 'divider', backgroundImage: t.brand.gradient.signatureSoft })}>
            <LogoMark size={28} />
          </Box>
          <Box>
            <Typography variant="h4" sx={{ fontSize: '1.7rem' }}>Goal &amp; constitution</Typography>
            <Typography sx={{ color: 'text.secondary' }}>The law of this project. The finisher and every agent read it before they build.</Typography>
          </Box>
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.3fr 1fr' }, gap: 2.5, alignItems: 'start' }}>
          {/* constitution editor */}
          <Card sx={{ p: 3 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Constitution</Typography>
            <TextField
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              multiline minRows={10} fullWidth
              placeholder={'e.g. A booking app for clinics. Patients book and pay; staff manage the calendar.\nRules: HIPAA-aligned · no PHI in logs · all writes audited · deploy only when Security + Compliance gates pass.'}
              sx={{ '& textarea': { fontSize: '0.95rem', lineHeight: 1.6 } }}
            />
            <Stack direction="row" spacing={1} sx={{ mt: 1.5, flexWrap: 'wrap', gap: 1 }}>
              {examples.map((ex) => (
                <Chip key={ex} label={ex} size="small" variant="outlined" onClick={() => setDraft(draft ? `${draft}\n${ex}` : ex)} sx={{ borderColor: 'divider', height: 'auto', py: 0.5, '& .MuiChip-label': { whiteSpace: 'normal' } }} />
              ))}
            </Stack>
            <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mt: 2.5 }}>
              <Button variant="contained" disabled={!dirty} onClick={() => { setConstitution(draft); toast('Constitution set — the chat is now grounded in it.', 'success'); }}>Set as constitution</Button>
              {!dirty && constitution ? <Typography sx={{ fontSize: '0.82rem', color: 'success.main' }}>● active</Typography> : dirty ? <Typography sx={{ fontSize: '0.82rem', color: 'text.disabled' }}>unsaved</Typography> : null}
            </Stack>
          </Card>

          {/* research / uploads */}
          <Card sx={{ p: 3 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Research &amp; references</Typography>

            <Box
              onClick={() => inputRef.current?.click()}
              onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => { e.preventDefault(); setDragging(false); void ingest(e.dataTransfer.files); }}
              sx={{ cursor: 'pointer', borderRadius: 3, border: '1.5px dashed', borderColor: dragging ? 'primary.main' : 'divider', bgcolor: dragging ? 'action.hover' : 'transparent', p: 3, textAlign: 'center', transition: (t) => `all ${t.brand.motion.fast}` }}
            >
              <Box sx={{ color: 'text.disabled', mb: 1 }}>
                <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12" /></svg>
              </Box>
              <Typography sx={{ fontSize: '0.9rem', fontWeight: 600 }}>Drop research, docs &amp; images</Typography>
              <Typography sx={{ fontSize: '0.78rem', color: 'text.disabled' }}>or click to browse — md, txt, json, png, jpg…</Typography>
              <input ref={inputRef} type="file" multiple hidden accept=".md,.markdown,.txt,.json,.csv,.yml,.yaml,.html,image/*" onChange={(e) => { void ingest(e.target.files); e.target.value = ''; }} />
            </Box>

            {attachments.length > 0 && (
              <Stack spacing={1} sx={{ mt: 2 }}>
                {attachments.map((a) => (
                  <Stack key={a.id} direction="row" alignItems="center" spacing={1.25} sx={{ p: 1, borderRadius: 2, border: 1, borderColor: 'divider' }}>
                    {a.kind === 'image' && a.dataUrl ? (
                      <Box component="img" src={a.dataUrl} alt={a.name} sx={{ width: 32, height: 32, borderRadius: 1, objectFit: 'cover' }} />
                    ) : (
                      <Box sx={{ width: 32, height: 32, borderRadius: 1, display: 'grid', placeItems: 'center', bgcolor: 'action.hover', color: 'text.secondary' }}>
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8zM14 2v6h6" /></svg>
                      </Box>
                    )}
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography sx={{ fontSize: '0.82rem', fontWeight: 600 }} noWrap>{a.name}</Typography>
                      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', color: 'text.disabled' })}>{a.kind}{a.text ? ' · grounds chat' : ''} · {fmtSize(a.size)}</Typography>
                    </Box>
                    <IconButton size="small" aria-label="Remove" onClick={() => removeAttachment(a.id)} sx={{ color: 'text.disabled' }}>✕</IconButton>
                  </Stack>
                ))}
              </Stack>
            )}

            <Typography sx={{ fontSize: '0.78rem', color: 'text.disabled', mt: 2 }}>
              Text docs are read and fed to the chat as context. Images and other files are attached as references.
            </Typography>
          </Card>
        </Box>
      </Box>
    </Box>
  );
}
