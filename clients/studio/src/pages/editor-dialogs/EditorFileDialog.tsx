import { useEffect, useState } from 'react';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, TextField, Typography } from '@mui/material';
import { text } from '@ironflyer/design-tokens/brand';

export type EditorFileDialogMode = 'create' | 'rename';

export function EditorFileDialog({
  open,
  mode,
  initialPath = '',
  existingPaths,
  onClose,
  onSubmit,
}: {
  open: boolean;
  mode: EditorFileDialogMode;
  initialPath?: string;
  existingPaths: string[];
  onClose: () => void;
  onSubmit: (path: string) => void;
}) {
  const [path, setPath] = useState(initialPath);

  useEffect(() => {
    if (open) setPath(initialPath);
  }, [initialPath, open]);

  const normalized = path.trim().replace(/^\/+/, '');
  const samePath = normalized === initialPath;
  const duplicate = normalized !== '' && !samePath && existingPaths.includes(normalized);
  const invalid = normalized === '' || normalized.endsWith('/') || duplicate;
  const helper = duplicate
    ? 'A file already exists at this path.'
    : normalized.endsWith('/')
      ? 'Use a full file path, including the file name.'
      : 'Use a workspace-relative path.';

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
    >
      <DialogTitle sx={{ fontWeight: 800, fontSize: text.s105 }}>
        {mode === 'create' ? 'New file' : 'Rename file'}
      </DialogTitle>
      <DialogContent dividers>
        <Stack spacing={1.5}>
          <Typography sx={{ color: 'text.secondary', fontSize: text.s78 }}>
            {mode === 'create'
              ? 'Create a file inside the current workspace tree.'
              : 'Move this file to a new workspace-relative path.'}
          </Typography>
          <TextField
            autoFocus
            fullWidth
            label="File path"
            value={path}
            error={invalid && normalized !== ''}
            helperText={helper}
            onChange={(e) => setPath(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !invalid) onSubmit(normalized);
              if (e.key === 'Escape') onClose();
            }}
          />
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 1.5 }}>
        <Button color="inherit" onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={invalid} onClick={() => onSubmit(normalized)}>
          {mode === 'create' ? 'Create' : 'Rename'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
