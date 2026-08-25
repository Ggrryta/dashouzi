const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ===== 类型定义 =====

export interface Domain {
  id: string;
  name: string;
  description: string;
  icon: string;
  checkpoint_count: number;
  scenario_count: number;
}

export interface Checkpoint {
  id: string;
  code: string;
  module: string;
  module_name: string;
  title: string;
  difficulty: number;
  frequency: string;
  tags: string[];
}

export interface Scenario {
  id: string;
  domain_id: string;
  title: string;
  difficulty: number;
  tags: string[];
  is_published: boolean;
  sort_order: number;
}

export interface StartSessionResponse {
  session_id: string;
  domain_id: string;
  scenario_title: string;
  background: {
    env: string;
    schema: string;
    data_volume: string;
    symptom: string;
  };
  sql_text: string;
  coach_message: string;
  current_level: number;
  total_levels: number;
}

export interface SubmitAnswerResponse {
  coach_message: string;
  is_correct: boolean;
  next_level: number;
  is_completed: boolean;
  note?: Note;
}

export interface Note {
  id: string;
  scenario_id: string;
  title: string;
  key_takeaways: string[];
}

// ===== API 调用 =====

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Unknown" }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  getDomains: () => fetchAPI<Domain[]>("/api/domains").then(r => r),

  getCheckpoints: (domain: string) =>
    fetchAPI<{ checkpoints: Checkpoint[]; total: number }>(
      `/api/checkpoints/${domain}`
    ),

  getScenarios: (domain: string) =>
    fetchAPI<{ scenarios: Scenario[]; total: number }>(
      `/api/scenarios/${domain}`
    ),

  startSession: (userID: string, scenarioID: string) =>
    fetchAPI<StartSessionResponse>("/api/coach/start", {
      method: "POST",
      body: JSON.stringify({ user_id: userID, scenario_id: scenarioID }),
    }),

  submitAnswer: (req: {
    session_id: string;
    user_id: string;
    scenario_id: string;
    answer: string;
    current_level: number;
    hints_used: number;
  }) =>
    fetchAPI<SubmitAnswerResponse>("/api/coach/answer", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getProgress: (userID: string, domain: string) =>
    fetchAPI<{ progress: Record<string, string> }>(
      `/api/progress/${userID}/${domain}`
    ),
};

// 临时用户ID（MVP阶段，后续接OAuth）
export const TEMP_USER_ID = "00000000-0000-0000-0000-000000000001";
