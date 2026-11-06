import sharp from "sharp";
const input = process.argv[2];
const output = process.argv[3];
await sharp(input).resize(512, 512).toFile(output);
console.log(`resized ${input} into ${output}`);
