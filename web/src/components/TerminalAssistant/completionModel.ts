// Validation only. Candidate text comes exclusively from the model.
export function validDraft(input: string): boolean {
  return !!input.trim() && input.length <= 4096 && !/[\x00-\x1f\x7f-\x9f]/.test(input);
}
export function validInsertion(text: string): boolean {
  return !!text && text.length <= 4096 && !/[\x00-\x1f\x7f-\x9f]/.test(text);
}
export class PromptBoundary {
  editing = false;
  blocked = true;
  column = -1;
  sequence(data: string, _line: number, column: number) {
    if (data === 'B') {
      this.editing = true; this.blocked = false; this.column = column;
    } else if (data === 'A' || data === 'C' || data === 'D' || data.startsWith('D;')) {
      this.editing = false; this.blocked = true;
    }
  }
  suspend() { this.blocked = true; }
}
