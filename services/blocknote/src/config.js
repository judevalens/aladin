// Central env parsing. Add collab/auth/admin config here in M8b/M8c.
export const config = {
  port: Number.parseInt(process.env.PORT ?? "3500", 10),
  jsonLimit: process.env.JSON_LIMIT ?? "10mb",
};
