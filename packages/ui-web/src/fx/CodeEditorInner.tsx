import CodeMirror from '@uiw/react-codemirror';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';

export default function CodeEditorInner({ value, language, height, dark, readOnly, onChange }: {
  value: string;
  language?: string;
  height: number | string;
  dark: boolean;
  readOnly?: boolean;
  onChange?: (v: string) => void;
}) {
  const ext = language === 'json' ? [json()] : [javascript({ jsx: true, typescript: true })];
  return (
    <CodeMirror
      value={value}
      height={typeof height === 'number' ? `${height}px` : height}
      theme={dark ? 'dark' : 'light'}
      extensions={ext}
      readOnly={readOnly}
      onChange={onChange}
      basicSetup={{ lineNumbers: true, foldGutter: true, highlightActiveLine: !readOnly }}
    />
  );
}
