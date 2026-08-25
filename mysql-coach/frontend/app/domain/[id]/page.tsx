"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, type Checkpoint, type Scenario, TEMP_USER_ID } from "@/lib/api";
import { motion } from "framer-motion";

export default function DomainPage() {
  const params = useParams();
  const router = useRouter();
  const domainID = params.id as string;

  const [checkpoints, setCheckpoints] = useState<Checkpoint[]>([]);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [progress, setProgress] = useState<Record<string, string>>({});
  const [tab, setTab] = useState<"checkpoints" | "scenarios">("scenarios");

  useEffect(() => {
    Promise.all([
      api.getCheckpoints(domainID),
      api.getScenarios(domainID),
      api.getProgress(TEMP_USER_ID, domainID),
    ]).then(([cp, sc, pr]) => {
      setCheckpoints(cp.checkpoints);
      setScenarios(sc.scenarios);
      setProgress(pr.progress);
    });
  }, [domainID]);

  const completedCount = Object.values(progress).filter(s => s === "completed").length;

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-purple-900 to-slate-900">
      <div className="max-w-4xl mx-auto px-4 py-8">
        {/* 返回 */}
        <button
          onClick={() => router.push("/")}
          className="text-slate-400 hover:text-white mb-6 text-sm"
        >
          ← 返回
        </button>

        {/* 标题 + 进度 */}
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-3xl font-bold text-white">{domainID.toUpperCase()}</h1>
          <div className="text-right">
            <div className="text-2xl font-bold text-cyan-400">{completedCount}</div>
            <div className="text-xs text-slate-500">已完成 / {checkpoints.length}</div>
          </div>
        </div>

        {/* 进度条 */}
        <div className="h-2 bg-slate-800 rounded-full mb-8 overflow-hidden">
          <motion.div
            className="h-full bg-gradient-to-r from-cyan-500 to-purple-500"
            initial={{ width: 0 }}
            animate={{ width: `${(completedCount / Math.max(checkpoints.length, 1)) * 100}%` }}
          />
        </div>

        {/* Tab 切换 */}
        <div className="flex gap-2 mb-6">
          <button
            onClick={() => setTab("scenarios")}
            className={`px-4 py-2 rounded-lg text-sm transition-colors ${
              tab === "scenarios" ? "bg-cyan-600 text-white" : "bg-slate-800 text-slate-400"
            }`}
          >
            场景训练 ({scenarios.length})
          </button>
          <button
            onClick={() => setTab("checkpoints")}
            className={`px-4 py-2 rounded-lg text-sm transition-colors ${
              tab === "checkpoints" ? "bg-cyan-600 text-white" : "bg-slate-800 text-slate-400"
            }`}
          >
            考点清单 ({checkpoints.length})
          </button>
        </div>

        {/* 场景列表 */}
        {tab === "scenarios" && (
          <div className="space-y-3">
            {scenarios.map((s, i) => (
              <motion.button
                key={s.id}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.05 }}
                whileHover={{ x: 4 }}
                onClick={() => router.push(`/train/${s.id}`)}
                className="w-full text-left bg-slate-800/50 backdrop-blur border border-slate-700 rounded-xl p-5 hover:border-cyan-500 transition-colors"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-yellow-500 text-sm">{"⭐".repeat(s.difficulty)}</span>
                      <h3 className="text-white font-semibold">{s.title}</h3>
                    </div>
                    <div className="flex gap-2 mt-2">
                      {s.tags?.map((t) => (
                        <span key={t} className="text-xs px-2 py-0.5 bg-slate-700 rounded text-slate-300">
                          {t}
                        </span>
                      ))}
                    </div>
                  </div>
                  <span className="text-cyan-400 text-xl">→</span>
                </div>
              </motion.button>
            ))}
          </div>
        )}

        {/* 考点清单 */}
        {tab === "checkpoints" && (
          <div className="space-y-2">
            {checkpoints.map((c, i) => {
              const done = progress[c.id] === "completed";
              return (
                <motion.div
                  key={c.id}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: i * 0.03 }}
                  className="flex items-center gap-3 bg-slate-800/50 border border-slate-700 rounded-lg px-4 py-3"
                >
                  <span className={`w-5 h-5 rounded-full flex items-center justify-center text-xs ${
                    done ? "bg-green-600 text-white" : "bg-slate-700 text-slate-500"
                  }`}>
                    {done ? "✓" : "○"}
                  </span>
                  <span className="text-slate-500 text-xs font-mono w-12">{c.code}</span>
                  <span className={`text-sm ${done ? "text-slate-400" : "text-white"}`}>
                    {c.title}
                  </span>
                  <span className="ml-auto text-xs text-slate-600">{c.frequency}</span>
                </motion.div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
