import { useEffect, useState } from 'react';
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from '@mui/material';
import { CodeEditor, toast } from '@ironflyer/ui-web/fx';
import { useThemeMode } from '@ironflyer/ui-web';
import type { Attachment } from '../store';

function langOf(name: string): string | undefined {
  if (/\.json$/i.test(name)) return 'json';
  return undefined;
}

// View/edit a single attachment in a dialog. Text docs (md/txt/json/csv/…) are
// editable via CodeMirror; images show inline; Word/Excel show a recommendation
// until the office engine is wired (see OnlyOffice/Univer note in the UI).
export function DocDialog({ attachment, onClose, onSave }: { attachment: Attachment | null; onClose: () => void; onSave: (id: string, text: string) => void }) {
  const { mode } = useThemeMode();
  const [draft, setDraft] = useState('');
  useEffect(() => { setDraft(attachment?.text ?? ''); }, [attachment]);
  if (!attachment) return null;

  const editable = attachment.text != null;
  const dirty = editable && draft !== attachment.text;

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography sx={{ fontWeight: 700, fontSize: '1.05rem' }} noWrap>{attachment.name}</Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })}>{attachment.kind}</Typography>
      </DialogTitle>
      <DialogContent dividers sx={{ p: editable ? 0 : 3 }}>
        {attachment.kind === 'image' && attachment.dataUrl ? (
          <Box component="img" src={attachment.dataUrl} alt={attachment.name} sx={{ width: '100%', borderRadius: 1 }} />
        ) : editable ? (
          <Box sx={{ '& .cm-editor': { maxHeight: 460 } }}>
            <CodeEditor value={draft} language={langOf(attachment.name)} dark={mode === 'dark'} height={460} onChange={setDraft} />
          </Box>
        ) : (
          <Box sx={{ textAlign: 'center', py: 4 }}>
            <Typography sx={{ color: 'text.secondary', mb: 1 }}>No in-app viewer for <b>{attachment.name}</b> yet.</Typography>
            <Typography sx={{ fontSize: '0.85rem', color: 'text.disabled' }}>Word/Excel view + edit lands when the office engine is wired (OnlyOffice or Univer).</Typography>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>Close</Button>
        {editable && (
          <Button variant="contained" disabled={!dirty} onClick={() => { onSave(attachment.id, draft); toast('Saved — research updated.', 'success'); onClose(); }}>Save</Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
