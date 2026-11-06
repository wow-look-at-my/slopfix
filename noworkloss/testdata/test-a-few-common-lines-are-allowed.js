const TOKEN = process.env.HF_TOKEN;
if (!TOKEN) throw new Error("HF_TOKEN is required");
const res = await fetch("https://huggingface.co/api/whoami-v2", { headers: { Authorization: `Bearer ${TOKEN}` } });
console.log(JSON.stringify(await res.json(), null, "\t"));
