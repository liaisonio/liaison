export type AgentEdgeID = string | number;
export function agentEdgeID(value: AgentEdgeID): AgentEdgeID {
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value) || value <= 0) throw new Error('INVALID_AGENT_CONNECTOR');
  } else if (typeof value !== 'string' || !/^[1-9][0-9]*$/.test(value) || BigInt(value) > 18446744073709551615n) {
    throw new Error('INVALID_AGENT_CONNECTOR');
  }
  return value;
}
