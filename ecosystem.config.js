module.exports = {
    apps: [
        {
            name: "sudevwa-api",
            script: "./sudevwa", // Ganti sudevwa.exe jika di Windows
            watch: false,
            env_file: ".env",
            instances: 1,
            exec_mode: "fork",
            max_memory_restart: "500M",
            autorestart: true,
            time: true, // Menambahkan timestamp di log terminal PM2
            env: {
                NODE_ENV: "production"
            }
        },
        {
            name: "sudevwa-worker",
            script: "./worker", // Output binary dari cmd/worker/main.go
            watch: false,
            env_file: ".env",
            instances: 1, // Cukup 1 instance karena Manager di dalamnya sudah handle banyak goroutine
            exec_mode: "fork",
            max_memory_restart: "500M", // Worker Go sangat irit RAM, 500M sangat aman
            autorestart: true,
            time: true,
            env: {
                NODE_ENV: "production"
            }
        }
    ]
}