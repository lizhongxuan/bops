export type StepSummary = {
  name: string;
  action: string;
  targets: string;
  when?: string;
  retries?: string;
  timeout?: string;
  loop?: string;
  notify?: string;
  env?: Record<string, string>;
  cmd?: string;
  dir?: string;
  script?: string;
  scriptRef?: string;
  scriptRefPresent?: boolean;
  withName?: string;
  src?: string;
  dest?: string;
  state?: string;
  required?: string;
  line?: number;
};

export function parseSteps(content: string) {
  const lines = content.split(/\r?\n/);
  const steps: StepSummary[] = [];
  let current: StepSummary | null = null;
  let inWith = false;
  let withIndent = 0;
  let inEnv = false;
  let envIndent = 0;
  let inScriptBlock = false;
  let scriptIndent = 0;
  let scriptLines: string[] = [];
  let inCmdBlock = false;
  let cmdIndent = 0;
  let cmdLines: string[] = [];
  let inTargets = false;
  let targetsIndent = 0;
  let targetItems: string[] = [];

  const flushScript = () => {
    if (current && scriptLines.length) {
      current.script = scriptLines.join("\n").trim();
    }
    scriptLines = [];
  };

  const flushCmd = () => {
    if (current && cmdLines.length) {
      current.cmd = cmdLines.join("\n").trim();
    }
    cmdLines = [];
  };

  const flushTargets = () => {
    if (current && targetItems.length) {
      current.targets = `[${targetItems.join(", ")}]`;
    }
    targetItems = [];
  };

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    const nameMatch = line.match(/^\s*-\s*name\s*:\s*(.+)$/);
    if (nameMatch) {
      if (inScriptBlock) {
        flushScript();
      }
      if (inCmdBlock) {
        flushCmd();
      }
      if (inTargets) {
        flushTargets();
      }
      current = {
        name: nameMatch[1].trim(),
        action: "",
        targets: "",
        cmd: "",
        dir: "",
        script: "",
        scriptRef: "",
        scriptRefPresent: false,
        withName: "",
        src: "",
        dest: "",
        state: "",
        required: "",
        line: index
      };
      steps.push(current);
      inWith = false;
      inEnv = false;
      inScriptBlock = false;
      inTargets = false;
      continue;
    }
    if (!current) {
      continue;
    }

    if (inCmdBlock) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (line.trim() === "") {
        cmdLines.push("");
        continue;
      }
      if (indent <= cmdIndent) {
        flushCmd();
        inCmdBlock = false;
      } else {
        cmdLines.push(line.slice(cmdIndent));
        continue;
      }
    }

    if (inScriptBlock) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (line.trim() === "") {
        scriptLines.push("");
        continue;
      }
      if (indent <= scriptIndent) {
        flushScript();
        inScriptBlock = false;
      } else {
        scriptLines.push(line.slice(scriptIndent));
        continue;
      }
    }

    if (inTargets) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (line.trim() === "") {
        continue;
      }
      if (indent <= targetsIndent) {
        flushTargets();
        inTargets = false;
      } else {
        const itemMatch = line.match(/^\s*-\s*(.+)$/);
        if (itemMatch) {
          targetItems.push(stripQuotes(itemMatch[1].trim()));
        }
        continue;
      }
    }

    const withMatch = line.match(/^(\s*)with\s*:\s*$/);
    if (withMatch) {
      inWith = true;
      withIndent = withMatch[1].length;
      inEnv = false;
      continue;
    }

    let handledWith = false;
    if (inWith) {
      const indent = line.match(/^(\s*)/)[1].length;
      if (line.trim() === "") {
        handledWith = true;
      } else if (indent <= withIndent) {
        inWith = false;
        inEnv = false;
      } else {
        handledWith = true;
        if (inEnv) {
          if (indent <= envIndent) {
            inEnv = false;
          } else {
            const envMatch = line.match(/^\s*([a-zA-Z0-9_-]+)\s*:\s*(.*)$/);
            if (envMatch) {
              if (!current.env) {
                current.env = {};
              }
              current.env[envMatch[1]] = stripQuotes(envMatch[2].trim());
            }
            continue;
          }
        }

        if (!inEnv) {
          const envMatch = line.match(/^\s*env\s*:\s*$/);
          if (envMatch) {
            inEnv = true;
            envIndent = indent;
            if (!current.env) {
              current.env = {};
            }
            continue;
          }

          const cmdMatch = line.match(/^\s*cmd\s*:\s*(.+)$/);
          if (cmdMatch) {
            const value = cmdMatch[1].trim();
            if (value.startsWith("|") || value.startsWith(">")) {
              inCmdBlock = true;
              cmdIndent = line.match(/^(\s*)/)[1].length + 2;
              cmdLines = [];
            } else {
              current.cmd = stripQuotes(value);
            }
          }
          const dirMatch = line.match(/^\s*dir\s*:\s*(.+)$/);
          if (dirMatch) {
            current.dir = stripQuotes(dirMatch[1].trim());
          }
          const withNameMatch = line.match(/^\s*name\s*:\s*(.+)$/);
          if (withNameMatch) {
            current.withName = stripQuotes(withNameMatch[1].trim());
          }
          const srcMatch = line.match(/^\s*src\s*:\s*(.+)$/);
          if (srcMatch) {
            current.src = stripQuotes(srcMatch[1].trim());
          }
          const destMatch = line.match(/^\s*dest\s*:\s*(.+)$/);
          if (destMatch) {
            current.dest = stripQuotes(destMatch[1].trim());
          }
          const stateMatch = line.match(/^\s*state\s*:\s*(.+)$/);
          if (stateMatch) {
            current.state = stripQuotes(stateMatch[1].trim());
          }
          const scriptRefMatch = line.match(/^\s*script_ref\s*:\s*(.*)$/);
          if (scriptRefMatch) {
            current.scriptRefPresent = true;
            current.scriptRef = stripQuotes(scriptRefMatch[1].trim());
          }
          const scriptMatch = line.match(/^\s*script\s*:\s*(.+)$/);
          if (scriptMatch) {
            const value = scriptMatch[1].trim();
            if (value.startsWith("|") || value.startsWith(">")) {
              inScriptBlock = true;
              scriptIndent = line.match(/^(\s*)/)[1].length + 2;
              scriptLines = [];
            } else {
              current.script = stripQuotes(value);
            }
          }
        }
      }
    }

    if (handledWith) {
      continue;
    }

    const actionMatch = line.match(/^\s*action\s*:\s*(.+)$/);
    if (actionMatch) {
      current.action = actionMatch[1].trim();
    }
    const targetsMatch = line.match(/^\s*targets\s*:\s*(.+)$/);
    if (targetsMatch) {
      current.targets = targetsMatch[1].trim();
    }
    const targetsBlockMatch = line.match(/^(\s*)targets\s*:\s*$/);
    if (targetsBlockMatch) {
      inTargets = true;
      targetsIndent = targetsBlockMatch[1].length;
      targetItems = [];
    }
    const whenMatch = line.match(/^\s*when\s*:\s*(.+)$/);
    if (whenMatch) {
      current.when = stripQuotes(whenMatch[1].trim());
    }
    const retriesMatch = line.match(/^\s*retries\s*:\s*(.+)$/);
    if (retriesMatch) {
      current.retries = stripQuotes(retriesMatch[1].trim());
    }
    const timeoutMatch = line.match(/^\s*timeout\s*:\s*(.+)$/);
    if (timeoutMatch) {
      current.timeout = stripQuotes(timeoutMatch[1].trim());
    }
    const loopMatch = line.match(/^\s*loop\s*:\s*(.+)$/);
    if (loopMatch) {
      current.loop = stripQuotes(loopMatch[1].trim());
    }
    const notifyMatch = line.match(/^\s*notify\s*:\s*(.+)$/);
    if (notifyMatch) {
      current.notify = stripQuotes(notifyMatch[1].trim());
    }
  }

  if (inScriptBlock) {
    flushScript();
  }
  if (inCmdBlock) {
    flushCmd();
  }
  if (inTargets) {
    flushTargets();
  }

  return steps.map((step) => ({
    ...step,
    required: buildRequiredSummary(step)
  }));
}

function buildRequiredSummary(step: StepSummary) {
  if (step.action === "cmd.run") {
    if (!step.cmd) {
      return "cmd: 未设置";
    }
    const firstLine = step.cmd.split(/\r?\n/)[0];
    return `cmd: ${truncateText(firstLine)}`;
  }
  if (step.action.startsWith("script.")) {
    if (step.scriptRefPresent) {
      if (step.scriptRef) {
        return `script_ref: ${step.scriptRef}`;
      }
      return "script_ref: 未设置";
    }
    if (step.script) {
      const firstLine = step.script.split(/\r?\n/)[0];
      return `script: ${truncateText(firstLine)}`;
    }
    return "script: 未设置";
  }
  if (step.action === "env.set") {
    const count = step.env ? Object.keys(step.env).length : 0;
    return count ? `env: ${count} 项` : "env: 未设置";
  }
  if (step.action === "pkg.install") {
    return step.withName ? `name: ${step.withName}` : "name: 未设置";
  }
  if (step.action === "template.render") {
    if (step.src && step.dest) {
      return `src: ${truncateText(step.src)} → ${truncateText(step.dest)}`;
    }
    if (step.src) {
      return `src: ${truncateText(step.src)}`;
    }
    if (step.dest) {
      return `dest: ${truncateText(step.dest)}`;
    }
    return "src/dest: 未设置";
  }
  if (step.action.startsWith("service.")) {
    if (!step.withName) {
      return "name: 未设置";
    }
    if (step.action === "service.ensure" && step.state) {
      return `svc: ${step.withName} (${step.state})`;
    }
    return `svc: ${step.withName}`;
  }
  return "";
}

function truncateText(value: string, max = 36) {
  const trimmed = value.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, max)}...`;
}

function stripQuotes(value: string) {
  return value.replace(/^['"]|['"]$/g, "");
}
