import { Terminal, Database, Shield, Zap, Copy, Check, Lock, Activity, HardDrive } from 'lucide-react';
import { useState } from 'react';

function App() {
	const [copiedInstall, setCopiedInstall] = useState(false);
	const [activeTab, setActiveTab] = useState<'backup' | 'restore' | 'config'>('backup');
	const [copiedRun, setCopiedRun] = useState(false);

	const copyToClipboard = (text: string, setter: (val: boolean) => void) => {
		navigator.clipboard.writeText(text);
		setter(true);
		setTimeout(() => setter(false), 2000);
	};

	const benchmarks = [
		{ name: 'Open Database connection', time: '1.1ms' },
		{ name: 'Parallel Deduplication (196MB)', time: '566ms' },
		{ name: 'Encrypt payload (AES-GCM)', time: '164ms' },
		{ name: 'Hash verification (SHA-256)', time: '521ms' },
		{ name: 'Target payload transfer', time: '2.85s' },
		{ name: 'Grandfather-Father-Son Prune', time: '14.1ms' },
		{ name: 'Total execution overhead', time: '4.12s' },
	];

	const features = [
		{ icon: <Database className="w-5 h-5 text-blue-400" />, title: 'Multi-Engine', desc: 'PostgreSQL, MySQL, SQLite supported seamlessly.' },
		{ icon: <Zap className="w-5 h-5 text-indigo-400" />, title: 'Built-in Dedupe', desc: 'Saves massive amounts of space with CAS hashing.' },
		{ icon: <Shield className="w-5 h-5 text-purple-400" />, title: 'Encrypted', desc: 'AES-256-GCM encryption on the client side.' },
		{ icon: <HardDrive className="w-5 h-5 text-pink-400" />, title: 'Multi-Cloud', desc: 'Store backups in S3, SFTP, FTP, or Locally.' },
	];

	return (
		<div className="min-h-screen bg-[#09090b] text-zinc-300 font-sans selection:bg-indigo-500/30 overflow-hidden">

			{/* Background gradients */}
			<div className="absolute top-0 left-0 w-full h-[500px] bg-gradient-to-b from-blue-900/10 via-indigo-900/5 to-transparent pointer-events-none" />
			<div className="absolute top-[-20%] right-[-10%] w-[50%] h-[50%] bg-blue-500/10 blur-[120px] rounded-full pointer-events-none" />
			<div className="absolute bottom-[-10%] left-[-10%] w-[40%] h-[40%] bg-purple-500/10 blur-[100px] rounded-full pointer-events-none" />

			{/* Header */}
			<header className="container mx-auto px-6 py-8 flex items-center justify-between relative z-10 border-b border-zinc-900">
				<div className="flex items-center gap-2">
					<div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center transform rotate-3">
						<Lock className="w-4 h-4 text-white" />
					</div>
					<span className="text-xl font-bold text-white tracking-tight">dbackup</span>
				</div>
				<div className="flex items-center gap-4">
					<a href="/docs/" className="flex items-center gap-2 text-sm text-zinc-300 hover:text-white transition-colors">
						Documentation
					</a>
					<a href="https://github.com/lupppig/dbackup" target="_blank" rel="noreferrer" className="flex items-center gap-2 text-sm text-zinc-400 hover:text-white transition-colors bg-white/5 hover:bg-white/10 px-4 py-2 rounded-full border border-zinc-800">
						<svg viewBox="0 0 24 24" className="w-4 h-4 fill-current"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" /></svg>
						View on GitHub
					</a>
				</div>
			</header>

			<main className="container mx-auto px-6 py-20 relative z-10">

				{/* Hero Section */}
				<section className="max-w-4xl mx-auto text-center mb-24">
					<div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-500/10 text-blue-400 text-xs font-medium border border-blue-500/20 mb-6">
						<span className="relative flex h-2 w-2">
							<span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
							<span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
						</span>
						v1.0 Ready for Production
					</div>

					<h1 className="text-5xl md:text-7xl font-bold text-white mb-6 leading-tight tracking-tighter">
						Smart backups, <br />
						<span className="gradient-text">beautifully executed.</span>
					</h1>

					<p className="text-lg md:text-xl text-zinc-400 mb-10 max-w-2xl mx-auto leading-relaxed">
						A high-performance CLI utility for database backups with built-in deduplication, AES-256 encryption, scheduling, and multi-cloud storage.
					</p>

					<div className="flex flex-col sm:flex-row items-center justify-center gap-4">
						<button
							onClick={() => copyToClipboard('go install github.com/lupppig/dbackup@latest', setCopiedInstall)}
							className="group relative inline-flex items-center gap-3 px-6 py-3 bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700 hover:border-zinc-600 rounded-lg text-zinc-200 font-mono text-sm transition-all duration-200"
						>
							<Terminal className="w-4 h-4 text-zinc-500 group-hover:text-blue-400" />
							<span>go install github.com/lupppig/dbackup@latest</span>
							{copiedInstall ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4 text-zinc-500 group-hover:text-zinc-300" />}
						</button>
					</div>
				</section>

				{/* Info Cards */}
				<section className="grid md:grid-cols-2 lg:grid-cols-4 gap-4 mb-24">
					{features.map((f, i) => (
						<div key={i} className="p-6 rounded-2xl bg-zinc-900/40 border border-zinc-800/50 hover:bg-zinc-800/50 transition-colors">
							<div className="w-10 h-10 rounded-lg bg-zinc-800 flex items-center justify-center mb-4">
								{f.icon}
							</div>
							<h3 className="text-white font-semibold mb-2">{f.title}</h3>
							<p className="text-sm text-zinc-400 leading-relaxed">{f.desc}</p>
						</div>
					))}
				</section>

				{/* Code & Demo Section */}
				<section className="grid lg:grid-cols-2 gap-12 items-center mb-24">
					<div>
						<h2 className="text-3xl font-bold text-white mb-4">Command it anywhere.</h2>
						<p className="text-zinc-400 mb-6 leading-relaxed">
							No complex setup required. dbackup takes a simple connection string and instantly handles extraction, chunk-based deduplication, compression, and secure storage in one seamless flow.
						</p>
						<ul className="space-y-3">
							{[
								'Parallel block discovery',
								'Advanced GFS Retention config',
								'Hash-chained audit logging',
								'Zero-leak credentials handling'
							].map((item, i) => (
								<li key={i} className="flex items-center gap-3 text-sm text-zinc-300">
									<div className="w-5 h-5 rounded-full bg-blue-500/20 flex items-center justify-center flex-shrink-0">
										<Check className="w-3 h-3 text-blue-400" />
									</div>
									{item}
								</li>
							))}
						</ul>
					</div>

					<div className="relative group rounded-xl p-[1px] bg-gradient-to-b from-zinc-700/50 to-zinc-900/50 overflow-hidden shadow-2xl shadow-blue-900/10 flex flex-col h-full">

						{/* Terminal Window Chrome */}
						<div className="bg-[#0c0c0e] rounded-xl flex flex-col h-full font-mono text-sm leading-relaxed overflow-hidden">

							{/* Terminal Header & Tabs */}
							<div className="flex items-center justify-between border-b border-zinc-800 bg-[#0c0c0e] px-4 pt-3">
								<div className="flex gap-2">
									<button
										onClick={() => setActiveTab('backup')}
										className={`px-4 py-2 text-xs font-semibold rounded-t-lg transition-colors ${activeTab === 'backup' ? 'bg-zinc-800/80 text-white' : 'text-zinc-500 hover:text-zinc-300'}`}
									>
										Backup
									</button>
									<button
										onClick={() => setActiveTab('restore')}
										className={`px-4 py-2 text-xs font-semibold rounded-t-lg transition-colors ${activeTab === 'restore' ? 'bg-zinc-800/80 text-white' : 'text-zinc-500 hover:text-zinc-300'}`}
									>
										Restore
									</button>
									<button
										onClick={() => setActiveTab('config')}
										className={`px-4 py-2 text-xs font-semibold rounded-t-lg transition-colors ${activeTab === 'config' ? 'bg-zinc-800/80 text-white' : 'text-zinc-500 hover:text-zinc-300'}`}
									>
										Config
									</button>
								</div>

								<div className="flex items-center gap-2 pb-2">
									<div className="w-3 h-3 rounded-full bg-red-500/80 border border-red-500"></div>
									<div className="w-3 h-3 rounded-full bg-yellow-500/80 border border-yellow-500"></div>
									<div className="w-3 h-3 rounded-full bg-green-500/80 border border-green-500"></div>
								</div>
							</div>

							{/* Terminal Content Body */}
							<div className="p-6 relative flex-grow">
								<div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity">
									<button
										onClick={() => {
											const text = activeTab === 'backup' ? 'dbackup backup postgres --db my_db --to s3://target' :
												activeTab === 'restore' ? 'dbackup restore mysql --from s3://backups/latest.manifest --to local_db' :
													'dbackup dump --config ~/.dbackup/backup.yaml';
											copyToClipboard(text, setCopiedRun);
										}}
										className="p-2 bg-zinc-800 hover:bg-zinc-700/80 rounded-md border border-zinc-700"
									>
										{copiedRun ? <Check className="w-3 h-3 text-green-400" /> : <Copy className="w-3 h-3 text-zinc-400" />}
									</button>
								</div>

								{activeTab === 'backup' && (
									<div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
										<div className="text-zinc-400 mb-1">
											<span className="text-indigo-400 font-bold">$</span> dbackup backup postgres \
										</div>
										<div className="text-zinc-400 mb-1 ml-4">
											--db my_db \
										</div>
										<div className="text-zinc-400 mb-4 ml-4">
											--to s3://key:secret@localhost:9000/backups
										</div>

										<div className="text-emerald-400/90 mb-1 text-xs">
											[INFO] Backup started engine=postgres database=my_db
										</div>
										<div className="text-zinc-500 mb-1 text-xs">
											[INFO] Deduplication (CAS) active
										</div>
										<div className="text-zinc-500 mb-1 text-xs">
											[INFO] Encrypting with AES-256-GCM...
										</div>
										<div className="text-blue-400/90 mt-2 text-xs font-semibold">
											✔ Backup saved successfully (total execution: 742ms)
										</div>
									</div>
								)}

								{activeTab === 'restore' && (
									<div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
										<div className="text-zinc-400 mb-1">
											<span className="text-indigo-400 font-bold">$</span> dbackup restore mysql \
										</div>
										<div className="text-zinc-400 mb-1 ml-4">
											--from s3://bucket/latest.manifest \
										</div>
										<div className="text-zinc-400 mb-4 ml-4">
											--to user:pass@localhost/dev_db --confirm-restore
										</div>

										<div className="text-emerald-400/90 mb-1 text-xs">
											[INFO] Reading manifest from S3...
										</div>
										<div className="text-zinc-500 mb-1 text-xs">
											[INFO] Decrypting payload blocks...
										</div>
										<div className="text-zinc-500 mb-1 text-xs">
											[INFO] Applying 2,491 records to MySQL target...
										</div>
										<div className="text-blue-400/90 mt-2 text-xs font-semibold">
											✔ Database restored successfully. Safe to start application.
										</div>
									</div>
								)}

								{activeTab === 'config' && (
									<div className="animate-in fade-in slide-in-from-bottom-2 duration-300 overflow-x-auto text-xs">
										<div className="text-zinc-400 mb-3">
											<span className="text-indigo-400 font-bold">$</span> cat ~/.dbackup/backup.json
										</div>
										<pre className="text-zinc-300 font-mono leading-relaxed">
											<span className="text-zinc-500">&#123;</span><br />
											&nbsp;&nbsp;<span className="text-pink-400">"parallelism"</span>: <span className="text-orange-300">4</span>,<br />
											&nbsp;&nbsp;<span className="text-pink-400">"backups"</span>: <span className="text-zinc-500">[</span><br />
											&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-zinc-500">&#123;</span><br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"id"</span>: <span className="text-green-300">"prod-db"</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"engine"</span>: <span className="text-green-300">"postgres"</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"uri"</span>: <span className="text-green-300">"postgres://user@localhost/prod"</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"to"</span>: <span className="text-green-300">"s3://bucket/backups"</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"dedupe"</span>: <span className="text-orange-300">true</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"encrypt"</span>: <span className="text-orange-300">true</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"retention"</span>: <span className="text-green-300">"30d"</span>,<br />
											&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-pink-400">"schedule"</span>: <span className="text-green-300">"0 2 * * *"</span><br />
											&nbsp;&nbsp;&nbsp;&nbsp;<span className="text-zinc-500">&#125;</span><br />
											&nbsp;&nbsp;<span className="text-zinc-500">]</span><br />
											<span className="text-zinc-500">&#125;</span>
										</pre>
									</div>
								)}
							</div>
						</div>
					</div>
				</section>

				{/* Benchmarks Section */}
				<section className="max-w-3xl mx-auto border border-zinc-800/50 rounded-2xl bg-zinc-900/20 backdrop-blur overflow-hidden relative">
					<div className="absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-zinc-600 to-transparent opacity-50"></div>

					<div className="p-6 md:p-8 flex flex-col md:flex-row items-center justify-between border-b border-zinc-800/50">
						<div>
							<h2 className="text-xl font-bold flex items-center gap-2 text-white">
								<Activity className="w-5 h-5 text-indigo-400" />
								Performance Benchmarks
							</h2>
							<p className="text-sm text-zinc-400 mt-1">Real-world execution using a 196MB SQLite database (1 Million Rows).</p>
						</div>
					</div>

					<div className="divide-y divide-zinc-800/50 font-mono text-sm bg-zinc-950/50">
						{benchmarks.map((bm, i) => (
							<div key={i} className="flex justify-between items-center p-4 hover:bg-zinc-800/30 transition-colors group">
								<span className="text-zinc-400 group-hover:text-zinc-300 transition-colors">{bm.name}</span>
								<span className={i === benchmarks.length - 1 ? "text-indigo-400 font-bold" : "text-zinc-500"}>{bm.time}</span>
							</div>
						))}
					</div>
				</section >

			</main >

			<footer className="container mx-auto px-6 py-12 text-center border-t border-zinc-900 mt-24">
				<p className="text-zinc-500 text-sm">
					Built securely with Go. Open source forever.
				</p>
			</footer>
		</div >
	);
}

export default App;
