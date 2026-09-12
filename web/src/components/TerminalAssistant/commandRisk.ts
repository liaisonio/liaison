// Display-only, best-effort warnings. This is not a shell parser, permission
// check or a source of completion candidates. Absence of a warning is not safety.
export function hasCommandRisk(command: string): boolean {
  const tokens = command.match(/"(?:\\.|[^"\\])*"|'[^']*'|[^\s;&|<>]+|[;&|<>]/g) || [];
  const words = tokens.map(t => t.replace(/^(['"])(.*)\1$/, '$2'));
  const destructive = /^(rm|rmdir|shred|mkfs(?:\..+)?|wipefs|dd|fdisk|sfdisk|parted|shutdown|reboot|halt|poweroff|kill|killall|pkill)$/;
  const riskySegment = (segment: string[]) => {
    let args = segment;
    // Wrapper arguments are not executable commands themselves.
    while (['sudo','doas','env','command','exec','nohup'].includes(args[0]?.split('/').pop() || '')) {
      const wrapper = args[0].split('/').pop();
      args = args.slice(1);
      while (args.length && (args[0].startsWith('-') || /^[A-Za-z_][\w]*=/.test(args[0]))) {
        const option = args.shift();
        if ((wrapper === 'sudo' && ['-u','-g','-h','-p'].includes(option!)) || (wrapper === 'doas' && option === '-u')) args = args.slice(1);
      }
    }
    const name = args[0]?.split('/').pop() || '';
    if (destructive.test(name)) return true;
    if (['chmod','chown','chgrp'].includes(name)) return true;
    if (name === 'git' && (args.includes('--hard') || args.includes('clean'))) return true;
    if (name === 'find' && (args.includes('-delete') || (args.some(a => a === '-exec' || a === '-execdir') && args.some(a => destructive.test(a.split('/').pop() || ''))))) return true;
    return ['sh','bash','zsh','dash'].includes(name) && (args.includes('-c') || words.includes('|'));
  };
  let segment: string[] = [];
  for (let i=0;i<words.length;i++) {
    if (tokens[i] === '>' && /^\/dev\//.test(words[i+1] || '') && !['/dev/null','/dev/stdout','/dev/stderr'].includes(words[i+1])) return true;
    if ([';','&','|'].includes(tokens[i])) {
      if (riskySegment(segment)) return true;
      segment=[];
    } else segment.push(words[i]);
  }
  return riskySegment(segment);
}
