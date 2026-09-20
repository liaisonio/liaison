import type {AgentModelSelection} from '@/services/agent';

// Model steps record the resolved provider/model used by that turn, not the
// current session preference (which may since have changed).
export function messageModel(detail: API.AgentSessionDetail, turnID: string): string | undefined {
  for (const step of detail.steps) {
    const selection=(step.input as {model_selection?:AgentModelSelection}|undefined)?.model_selection;
    if(step.turn_id===turnID && selection?.model)return selection.model;
  }
}
