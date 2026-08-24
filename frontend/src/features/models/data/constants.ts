export const DEVELOPER_IDS = [
  'deepseek',
  'alibaba',
  'tencent',
  'zai',
  'openai',
  'moonshot',
  'anthropic',
  'google',
  'minimax',
  'kwaipilot',
  'xiaomi',
  'longcat',
  'mistral',
  'nvidia',
  'xai',
  'bytedance',
  'stepfun',
  'meta',
  'ibm',
  'poolside',
  'inclusionai',
  'thinkingmachines',
];

export const DEVELOPER_ICONS: Record<string, string> = {
  moonshot: 'Moonshot',
  xai: 'XAI',
  zai: 'ZAI',
  deepseek: 'DeepSeek',
  google: 'Gemini',
  anthropic: 'Claude',
  openai: 'OpenAI',
  minimax: 'Minimax',
  kwaipilot: 'KwaiKAT',
  alibaba: 'Qwen',
  tencent: 'Hunyuan',
  xiaomi: 'XiaomiMiMo',
  longcat: 'LongCat',
  mistral: 'Mistral',
  nvidia: 'Nvidia',
  bytedance: 'Doubao',
  stepfun: 'Stepfun',
  meta: 'Meta',
  ibm: 'IBM',
};

// MAX_ASSOCIATIONS caps how many association rules one Model may hold. It mirrors
// MaxModelAssociations in internal/server/biz/model_import.go, which rejects
// imports that would exceed it, so the UI must not offer a target already at the cap.
export const MAX_ASSOCIATIONS = 10;
