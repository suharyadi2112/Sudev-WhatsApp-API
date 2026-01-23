module.exports = {
    apps: [
        {
            name: "sudevwa-api", // Nama aplikasi di PM2
            // script: "./sudevwa.exe", // Binary untuk Windows
            script: "./sudevwa",   // Binary untuk Ubuntu/Linux
            watch: false, // Auto-restart saat file berubah (false = production)
            env_file: ".env", // Load environment variables dari file .env
            instances: 1, // Jumlah instance (1 = single process, "max" = sesuai CPU core)
            exec_mode: "fork", // Mode eksekusi ("fork" = standar, "cluster" = Node.js cluster)
            max_memory_restart: "500M", // Auto restart jika memory > 500MB
            autorestart: true, // Auto restart jika aplikasi crash
            max_restarts: 10, // Maksimal restart berturut-turut sebelum PM2 stop
            min_uptime: "10s", // Minimum waktu hidup agar dianggap "stable"
            env: {
                NODE_ENV: "production" // Environment variables tambahan
            }
        }
    ]
}