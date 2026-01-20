export type StepWith = {
  cmd?: string;
  dir?: string;
  name?: string;
  state?: string;
  src?: string;
  dest?: string;
  script?: string;
  scriptRef?: string;
  packages?: string;
  envText?: string;
};

export type DraftStep = {
  id: string;
  name: string;
  action: string;
  targets: string;
  with: StepWith;
};

export function createDefaultStepWith(action?: string): StepWith {
  return {
    cmd: "",
    dir: "",
    name: "",
    state: action === "service.ensure" ? "started" : "",
    src: "",
    dest: "",
    script: "",
    scriptRef: "",
    packages: "",
    envText: ""
  };
}
