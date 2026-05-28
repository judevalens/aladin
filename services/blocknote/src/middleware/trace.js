// Structured-ish request logging. One line per completed request.
export function trace(req, res, next) {
  const start = Date.now();
  res.on("finish", () => {
    console.log(
      `${req.method} ${req.path} ${res.statusCode} ${Date.now() - start}ms`,
    );
  });
  next();
}
