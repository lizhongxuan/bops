const sseStream = `event: status
` +
  `data: {"node":"generator","status":"start"}\n\n` +
  `event: message\n` +
  `data: {"message_id":"loop-1-1-start","reply_id":"reply-1","role":"assistant","type":"function_call","content":"search config","is_finish":false,"index":1,"extra_info":{"call_id":"search_file","execute_display_name":"{\\"name_executing\\":\\"正在搜索文件\\",\\"name_executed\\":\\"已完成搜索文件\\",\\"name_execute_failed\\":\\"搜索文件失败\\"}","loop_id":"loop-1","iteration":1,"agent_status":"tool_call"}}\n\n` +
  `event: delta\n` +
  `data: {"channel":"reasoning","content":"思考中..."}\n\n` +
  `event: delta\n` +
  `data: {"channel":"answer","content":"这是一段回答"}\n\n` +
  `event: message\n` +
  `data: {"message_id":"loop-1-1-done","reply_id":"reply-1","role":"assistant","type":"tool_response","content":"done","is_finish":true,"index":2,"extra_info":{"call_id":"search_file","execute_display_name":"{\\"name_executing\\":\\"正在搜索文件\\",\\"name_executed\\":\\"已完成搜索文件\\",\\"name_execute_failed\\":\\"搜索文件失败\\"}","loop_id":"loop-1","iteration":1,"agent_status":"tool_result"}}\n\n` +
  `event: message\n` +
  `data: {"message_id":"loop-1-2-start","reply_id":"reply-1","role":"assistant","type":"function_call","content":"write file","is_finish":false,"index":3,"extra_info":{"call_id":"write_file","execute_display_name":"{\\"name_executing\\":\\"正在创建文件\\",\\"name_executed\\":\\"已完成创建文件\\",\\"name_execute_failed\\":\\"创建文件失败\\"}","loop_id":"loop-1","iteration":2,"agent_status":"tool_call"}}\n\n` +
  `event: message\n` +
  `data: {"message_id":"loop-1-2-stream","reply_id":"reply-1","role":"assistant","type":"tool_response","content":"streaming...","is_finish":false,"index":4,"extra_info":{"call_id":"write_file","stream_plugin_running":"stream-123","loop_id":"loop-1","iteration":2,"agent_status":"tool_result"}}\n\n` +
  `event: message\n` +
  `data: {"message_id":"loop-1-2-verbose","reply_id":"reply-1","role":"assistant","type":"verbose","content":"{\\"msg_type\\":\\"stream_plugin_finish\\",\\"data\\":\\"{\\\\\\"uuid\\\\\\":\\\\\\"stream-123\\\\\\",\\\\\\"tool_output_content\\\\\\":\\\\\\"stream done\\\\\\"}\\"}","is_finish":true,"index":5,"extra_info":{"call_id":"write_file","stream_plugin_running":"stream-123","loop_id":"loop-1","iteration":2,"agent_status":"tool_result"}}\n\n` +
  `event: card\n` +
  `data: {"card_type":"file_create","title":"创建文件","files":[{"path":"workflow.yaml","content":"version: v0.1"}]}\n\n` +
  `event: result\n` +
  `data: {"summary":"ok","plan":[{"step_name":"install","description":"install nginx","dependencies":[]}],"subagent_summaries":[{"agent_name":"reviewer","summary":"issues=0"}]}\n\n`;

const chatEntries = [];
let chatIndex = 0;
let liveAnswerEntryId = null;
let liveReasoning = "";
const functionCallEntryIds = {};
const functionCallStreamEntryIds = {};
let sawStreamRunning = false;
let sawStreamFinish = false;

function pushChatEntry(entry) {
  const id = `chat-${chatIndex++}`;
  chatEntries.push({ id, ...entry });
  return id;
}

function updateChatEntry(id, updater) {
  const index = chatEntries.findIndex((entry) => entry.id === id);
  if (index < 0) return;
  chatEntries[index] = updater(chatEntries[index]);
}

function handleFunctionCallMessage(msg) {
  const callId = msg?.extra_info?.call_id || msg.message_id;
  const loopId = msg?.extra_info?.loop_id || "";
  const iteration = typeof msg?.extra_info?.iteration === "number" ? msg.extra_info.iteration : undefined;
  const groupKey = loopId && iteration ? `loop:${loopId}:${iteration}` : callId;
  const isFinish = typeof msg.is_finish === "boolean" ? msg.is_finish : msg.type !== "function_call";
  const isRunning = msg.type === "function_call" || (msg.type === "tool_response" && !isFinish);
  const status = isRunning ? "running" : "done";
  const streamUuid = msg.extra_info?.stream_plugin_running || "";
  const unit = {
    callId,
    title: msg.content || callId,
    status,
    content: msg.content,
    streamUuid: streamUuid || undefined,
    loopId: loopId || undefined,
    iteration
  };
  if (msg.type === "tool_response" && msg.extra_info?.stream_plugin_running && status === "running") {
    sawStreamRunning = true;
  }
  const entryId = functionCallEntryIds[groupKey];
  if (entryId) {
    updateChatEntry(entryId, (entry) => ({ ...entry, functionCalls: [unit] }));
    if (streamUuid) {
      functionCallStreamEntryIds[streamUuid] = entryId;
    }
    return;
  }
  const label = iteration ? `第 ${iteration} 轮` : "步骤";
  const newId = pushChatEntry({ label, body: "", type: "function_call", functionCalls: [unit] });
  functionCallEntryIds[groupKey] = newId;
  if (streamUuid) {
    functionCallStreamEntryIds[streamUuid] = newId;
  }
}

function handleVerboseMessage(msg) {
  if (msg.type !== "verbose") return;
  if (!msg.content) return;
  let parsed = null;
  try {
    parsed = JSON.parse(msg.content);
  } catch {
    return;
  }
  if (!parsed || typeof parsed.msg_type !== "string") return;
  if (parsed.msg_type !== "stream_plugin_finish") return;
  let dataObj = null;
  if (typeof parsed.data === "string") {
    try {
      dataObj = JSON.parse(parsed.data);
    } catch {
      dataObj = null;
    }
  }
  const streamUuid = dataObj?.uuid || msg.extra_info?.stream_plugin_running;
  if (!streamUuid) return;
  const entryId = functionCallStreamEntryIds[streamUuid];
  if (!entryId) return;
  sawStreamFinish = true;
  updateChatEntry(entryId, (entry) => {
    const unit = entry.functionCalls?.[0] || { callId: streamUuid, title: streamUuid };
    return {
      ...entry,
      functionCalls: [{ ...unit, status: "done", content: dataObj?.tool_output_content || unit.content }]
    };
  });
}

function appendAnswerDelta(delta) {
  if (!delta) return;
  let id = liveAnswerEntryId;
  if (!id) {
    id = pushChatEntry({ label: "AI", body: "", type: "ai" });
    liveAnswerEntryId = id;
  }
  updateChatEntry(id, (entry) => ({ ...entry, body: `${entry.body || ""}${delta}` }));
}

function appendReasoningDelta(delta) {
  if (!delta) return;
  let id = liveAnswerEntryId;
  if (!id) {
    id = pushChatEntry({ label: "AI", body: "", type: "ai", reasoning: "" });
    liveAnswerEntryId = id;
  }
  liveReasoning = `${liveReasoning}${delta}`;
  updateChatEntry(id, (entry) => ({ ...entry, reasoning: liveReasoning }));
}

function handleSSEChunk(chunk) {
  const lines = chunk.split("\n");
  let eventName = "message";
  let data = "";
  for (const line of lines) {
    if (line.startsWith("event:")) {
      eventName = line.replace("event:", "").trim();
    } else if (line.startsWith("data:")) {
      data += line.replace("data:", "").trim();
    }
  }
  if (!data) return;
  const payload = JSON.parse(data);
  if (eventName === "message") {
    if (payload.type === "verbose") {
      handleVerboseMessage(payload);
    } else {
      handleFunctionCallMessage(payload);
    }
  } else if (eventName === "card") {
    pushChatEntry({ label: "卡片", body: "", type: "card", card: payload });
  } else if (eventName === "delta") {
    if (payload.channel === "reasoning") {
      appendReasoningDelta(payload.content || "");
    } else if (payload.channel === "answer") {
      appendAnswerDelta(payload.content || "");
    }
  } else if (eventName === "result") {
    if (Array.isArray(payload.plan) && payload.plan.length) {
      pushChatEntry({ label: "计划", body: "plan", type: "ai" });
    }
    if (Array.isArray(payload.subagent_summaries) && payload.subagent_summaries.length) {
      pushChatEntry({ label: "子 Agent 汇总", body: "agents", type: "ai" });
    }
  }
}

for (const chunk of sseStream.split("\n\n")) {
  if (chunk.trim()) handleSSEChunk(chunk);
}

const hasAnswer = chatEntries.some((entry) => entry.type === "ai" && entry.body.includes("回答"));
const hasReasoning = chatEntries.some((entry) => entry.type === "ai" && (entry.reasoning || "").includes("思考"));
const hasSteps = chatEntries.some((entry) => entry.type === "function_call" && (entry.functionCalls || []).length > 0);
const loopEntries = chatEntries.filter((entry) => entry.type === "function_call");
const hasSecondIteration = loopEntries.some((entry) => (entry.label || "").includes("第 2 轮"));
const hasFileCard = chatEntries.some(
  (entry) => entry.type === "card" && entry.card?.card_type === "file_create"
);
const hasPlan = chatEntries.some((entry) => entry.label === "计划");
const hasSummaries = chatEntries.some((entry) => entry.label === "子 Agent 汇总");

if (!hasAnswer) {
  console.error("E2E failed: answer delta missing");
  process.exit(1);
}
if (!hasReasoning) {
  console.error("E2E failed: reasoning delta missing");
  process.exit(1);
}
if (!sawStreamRunning) {
  console.error("E2E failed: stream_plugin_running not handled as running");
  process.exit(1);
}
if (!sawStreamFinish) {
  console.error("E2E failed: stream_plugin_finish verbose not handled");
  process.exit(1);
}
if (!hasSteps) {
  console.error("E2E failed: function calls missing");
  process.exit(1);
}
if (loopEntries.length < 2 || !hasSecondIteration) {
  console.error("E2E failed: loop iteration grouping missing");
  process.exit(1);
}
if (!hasFileCard) {
  console.error("E2E failed: card entries missing");
  process.exit(1);
}
if (!hasPlan) {
  console.error("E2E failed: plan entry missing");
  process.exit(1);
}
if (!hasSummaries) {
  console.error("E2E failed: subagent summaries missing");
  process.exit(1);
}

console.log("E2E stream test passed");
