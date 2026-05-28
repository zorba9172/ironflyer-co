// Copy guardrails — keep marketing + product strings reading like a person,
// not an LLM. See BRAND_SYSTEM §6.

export const bannedPhrases: readonly string[] = [
  'unlock',
  'unleash',
  'elevate',
  'seamless',
  'seamlessly',
  'revolutionize',
  'revolutionary',
  'game-changer',
  'game changer',
  'in today’s fast-paced',
  'in todays fast-paced',
  'cutting-edge',
  'state-of-the-art',
  'leverage the power',
  'take it to the next level',
  'supercharge',
  'effortlessly',
  'whether you’re',
  'look no further',
  'dive in',
  'delve',
  'embark on a journey',
  'we’ve got you covered',
];

export interface CopyLintHit {
  phrase: string;
  index: number;
}

// Flags AI-tell phrases in a string. Use in CI/empty-state review.
export function lintCopy(text: string): CopyLintHit[] {
  const lower = text.toLowerCase();
  const hits: CopyLintHit[] = [];
  for (const phrase of bannedPhrases) {
    let from = 0;
    for (;;) {
      const at = lower.indexOf(phrase, from);
      if (at === -1) break;
      hits.push({ phrase, index: at });
      from = at + phrase.length;
    }
  }
  return hits.sort((a, b) => a.index - b.index);
}
