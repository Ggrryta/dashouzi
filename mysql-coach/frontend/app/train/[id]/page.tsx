"use client";

import { useState, useRef, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, TEMP_USER_ID, type StartSessionResponse, type SubmitAnswerResponse } from "@/lib/api";
import { motion, AnimatePresence } from "framer-motion";
import ReactMarkdown from "react-markdown";

interface Message {
  role: "coach" | "student";
  content: string;
}

export default function TrainPage() {
  const params = useParams();
  const router = useRouter();
  const scenarioID = params.id as string;

  const [session, setSession] = useState<StartSessionResponse | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [completed, setCompleted] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 开始训练
  useEffect(() => {
    api.startSession(TEMP_USER_ID, scenarioID)
      .then((resp) => {
        setSession(resp);
        setMessages([{ role: "coach", content: resp.coach_message }]);
      })
      .catch(e => setError(e.message));
  }, [scenarioID]);

  // 自动滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // 提交回答
  const handleSubmit = async () => {
    if (!input.trim() || !session || loading) return;
    setLoading(true);
    const answer = input.trim();
    setInput("");

    // 先显示学生回答
    setMessages(prev => [...prev, { role: "student", content: answer }]);

    try {
      const resp: SubmitAnswerResponse = await api.submitAnswer({
        session_id: session.session_id,
        user_id: TEMP_USER_ID,
        scenario_id: scenarioID,
        answer,
        current_level: session.current_level,
        hints_used: 0,
      });

      // 显示教练回复
      setMessages(prev => [...prev, { role: "coach", content: resp.coach_message }]);

      // 更新当前层级
      if (resp.next_level > 0) {
        setSession({ ...session, current_level: resp.next_level });
      }
      if (resp.is_completed) {
        setCompleted(true);
      }
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  if (error) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-red-400">错误：{error}</div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-slate-400">加载场景中...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-900 flex flex-col">
      {/* 顶部导航 */}
      <div className="bg-slate-800/50 backdrop-blur border-b border-slate-700 px-4 py-3 flex items-center gap-4">
        <button onClick={() => router.back()} className="text-slate-400 hover:text-white text-sm">
          ← 返回
        </button>
        <div className="flex-1">
          <h1 className="text-white font-semibold">{session.scenario_title}</h1>
        </div>
        <div className="flex items-center gap-2 text-sm">
          {Array.from({ length: session.total_levels }).map((_, i) => (
            <div
              key={i}
              className={`w-6 h-6 rounded-full flex items-center justify-center text-xs ${
                i < session.current_level - 1
                  ? "bg-green-600 text-white"
                  : i === session.current_level - 1
                  ? "bg-cyan-600 text-white animate-pulse"
                  : "bg-slate-700 text-slate-500"
              }`}
            >
              {i + 1}
            </div>
          ))}
        </div>
      </div>

      {/* 对话区域 */}
      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-4 max-w-3xl mx-auto w-full">
        {messages.map((msg, i) => (
          <motion.div
            key={i}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className={`flex ${msg.role === "student" ? "justify-end" : "justify-start"}`}
          >
            <div
              className={`max-w-[85%] rounded-2xl px-4 py-3 ${
                msg.role === "student"
                  ? "bg-cyan-600 text-white"
                  : "bg-slate-800 text-slate-100 border border-slate-700"
              }`}
            >
              {msg.role === "coach" ? (
                <div className="prose prose-invert prose-sm max-w-none">
                  <ReactMarkdown>{msg.content}</ReactMarkdown>
                </div>
              ) : (
                <p className="text-sm">{msg.content}</p>
              )}
            </div>
          </motion.div>
        ))}

        {/* 加载中 */}
        {loading && (
          <div className="flex justify-start">
            <div className="bg-slate-800 border border-slate-700 rounded-2xl px-4 py-3">
              <div className="flex gap-1">
                <span className="w-2 h-2 bg-slate-500 rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
                <span className="w-2 h-2 bg-slate-500 rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
                <span className="w-2 h-2 bg-slate-500 rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
              </div>
            </div>
          </div>
        )}

        {/* 训练完成 */}
        <AnimatePresence>
          {completed && (
            <motion.div
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              className="bg-green-900/30 border border-green-700 rounded-2xl p-6 text-center"
            >
              <div className="text-4xl mb-2">🎉</div>
              <h3 className="text-xl font-bold text-green-400 mb-2">训练完成！</h3>
              <p className="text-slate-400 text-sm mb-4">笔记已自动沉淀，考点已打勾</p>
              <div className="flex gap-3 justify-center">
                <button
                  onClick={() => router.push(`/domain/mysql`)}
                  className="px-4 py-2 bg-slate-700 rounded-lg text-sm text-white hover:bg-slate-600"
                >
                  查看进度
                </button>
                <button
                  onClick={() => router.push("/")}
                  className="px-4 py-2 bg-cyan-600 rounded-lg text-sm text-white hover:bg-cyan-500"
                >
                  返回首页
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <div ref={messagesEndRef} />
      </div>

      {/* 输入区 */}
      {!completed && (
        <div className="border-t border-slate-700 bg-slate-800/50 backdrop-blur px-4 py-4">
          <div className="max-w-3xl mx-auto flex gap-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSubmit();
                }
              }}
              placeholder="输入你的分析..."
              rows={2}
              className="flex-1 bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-white text-sm resize-none focus:outline-none focus:border-cyan-500"
              disabled={loading}
            />
            <button
              onClick={handleSubmit}
              disabled={loading || !input.trim()}
              className="px-6 py-3 bg-gradient-to-r from-cyan-600 to-purple-600 rounded-xl text-white text-sm font-medium hover:opacity-90 disabled:opacity-50"
            >
              发送
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
