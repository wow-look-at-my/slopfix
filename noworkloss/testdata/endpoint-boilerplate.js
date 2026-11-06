const NAME = "qwen38-flash-next-nvfp4";
const TOKEN = process.env.HF_TOKEN;
if (!TOKEN) throw new Error("HF_TOKEN is required");
const URL = `https://api.endpoints.huggingface.cloud/v2/endpoint/alternateraise/${NAME}`;
const headers = { Authorization: `Bearer ${TOKEN}`, "Content-Type": "application/json" };

const getRes = await fetch(URL, { headers });
if (!getRes.ok) throw new Error(`GET ${getRes.status}: ${await getRes.text()}`);
const model = (await getRes.json()).model;
