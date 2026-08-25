"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, type Domain } from "@/lib/api";
import { motion } from "framer-motion";

export default function Home() {
  const router = useRouter();
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getDomains().then(setDomains).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-purple-900 to-slate-900">
      {/* Hero */}
      <div className="max-w-5xl mx-auto pt-20 pb-12 px-4 text-center">
        <motion.h1
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-5xl font-bold bg-gradient-to-r from-cyan-400 to-purple-400 bg-clip-text text-transparent"
        >
          MySQL Coach
        </motion.h1>
        <motion.p
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.2 }}
          className="mt-4 text-xl text-slate-300"
        >
          不是刷题，是陪练。不是背答案，是被追问。
        </motion.p>
      </div>

      {/* 领域卡片 */}
      <div className="max-w-4xl mx-auto px-4 pb-20">
        <h2 className="text-lg text-slate-400 mb-6">选择训练领域</h2>
        {loading ? (
          <div className="text-slate-400">加载中...</div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {domains.map((d, i) => (
              <motion.button
                key={d.id}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.1 }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={() => router.push(`/domain/${d.id}`)}
                className="bg-slate-800/50 backdrop-blur border border-slate-700 rounded-2xl p-6 text-left hover:border-cyan-500 transition-colors"
              >
                <div className="text-4xl mb-3">{d.icon}</div>
                <h3 className="text-xl font-bold text-white mb-1">{d.name}</h3>
                <p className="text-sm text-slate-400 mb-4">{d.description}</p>
                <div className="flex gap-4 text-xs text-slate-500">
                  <span>{d.checkpoint_count} 考点</span>
                  <span>{d.scenario_count} 场景</span>
                </div>
              </motion.button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
