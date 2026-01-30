const sseStream = `event: status
` +
  `data: {"node":"generator","status":"start"}\n\n` +
  `event: message\n` +
  `data: {"message_id":"generator-start-1","reply_id":"reply-1","role":"assistant","type":"function_call","content":"start","is_finish":false,"index":1,"extra_info":{"call_id":"generator","execute_display_name":"{\\"name_executing\\":\\"正在生成工作流\\",\\"name_executed\\":\\"已完成生成工作流\\",\\"name_execute_failed\\":\\"生成工作流失败\\"}"}}\n\n` +
  `event: delta\n` +
  `data: {"channel":"thought","content":"思考中..."}\n\n` +
  `event: delta\n` +
  `data: {"channel":"answer","content":"这是一段回答"}\n\n` +
  `event: message\n` +
  `data: {"message_id":"generator-done-2","reply_id":"reply-1","role":"assistant","type":"tool_response","content":"done","is_finish":true,"index":2,"extra_info":{"call_id":"generator","execute_display_name":"{\\"name_executing\\":\\"正在生成工作流\\",\\"name_executed\\":\\"已完成生成工作流\\",\\"name_execute_failed\\":\\"生成工作流失败\\"}"}}\n\n` +
  `event: card\n` +
  `data: {"card_type":"create_step","step":{"name":"生成步骤","action":"cmd.run"}}\n\n` +
  `event: card\n` +
  `data: {"card_type":"file_create","title":"创建文件","files":[{"path":"workflow.yaml","content":"version: v0.1"}]}\n\n` +
  `event: result\n` +
  `data: {"summary":"ok"}\n\n`;

const chatEntries = [];
let chatIndex = 0;
let liveAnswerEntryId = null;
const functionCallEntryIds = {};

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
  const isRunning = msg.type === "function_call";
  const status = isRunning ? "running" : "done";
  const unit = {
    callId,
    title: msg.content || callId,
    status,
    content: msg.content
  };
  const entryId = functionCallEntryIds[callId];
  if (entryId) {
    updateChatEntry(entryId, (entry) => ({ ...entry, functionCalls: [unit] }));
    return;
  }
  const newId = pushChatEntry({ label: "步骤", body: "", type: "function_call", functionCalls: [unit] });
  functionCallEntryIds[callId] = newId;
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
    handleFunctionCallMessage(payload);
  } else if (eventName === "card") {
    pushChatEntry({ label: "卡片", body: "", type: "card", card: payload });
  } else if (eventName === "delta") {
    if (payload.channel === "answer") {
      appendAnswerDelta(payload.content || "");
    }
  }
}

for (const chunk of sseStream.split("\n\n")) {
  if (chunk.trim()) handleSSEChunk(chunk);
}

const hasAnswer = chatEntries.some((entry) => entry.type === "ai" && entry.body.includes("回答"));
const hasSteps = chatEntries.some((entry) => entry.type === "function_call" && (entry.functionCalls || []).length > 0);
const hasFileCard = chatEntries.some(
  (entry) => entry.type === "card" && entry.card?.card_type === "file_create"
);
const hasCreateStep = chatEntries.some(
  (entry) => entry.type === "card" && entry.card?.card_type === "create_step"
);

if (!hasAnswer) {
  console.error("E2E failed: answer delta missing");
  process.exit(1);
}
if (!hasSteps) {
  console.error("E2E failed: function calls missing");
  process.exit(1);
}
if (!hasFileCard || !hasCreateStep) {
  console.error("E2E failed: card entries missing");
  process.exit(1);
}

console.log("E2E stream test passed");
