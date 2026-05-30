import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from '@mui/material';
import { text } from '@ironflyer/design-tokens/brand';

export function EditorConfirmDialog({
  open,
  title,
  text: body,
  confirmText = 'Confirm',
  danger,
  onClose,
  onConfirm,
}: {
  open: boolean;
  title: string;
  text: string;
  confirmText?: string;
  danger?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
      slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
    >
      <DialogTitle sx={{ fontWeight: 800, fontSize: text.s105 }}>{title}</DialogTitle>
      <DialogContent dividers>
        <Typography sx={{ color: 'text.secondary', fontSize: text.s82, lineHeight: 1.55 }}>{body}</Typography>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 1.5 }}>
        <Button color="inherit" onClick={onClose}>Cancel</Button>
        <Button variant="contained" color={danger ? 'error' : 'primary'} onClick={onConfirm}>
          {confirmText}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
