import { Handle, Position } from '@xyflow/react';
import type { NodeProps } from '@xyflow/react';
import { Database, FileCode, Layers } from 'lucide-react';

export default function CustomNode({ data }: NodeProps) {
  const isView = data.type === 'view';
  const isTable = data.type === 'table';
  const label = String(data.label || '');
  const typeStr = String(data.type || 'model');
  
  return (
    <div className="relative p-4 rounded-xl backdrop-blur-xl bg-white/10 border border-white/20 shadow-xl min-w-[200px] hover:bg-white/20 transition-all duration-300 group">
      <Handle type="target" position={Position.Top} className="w-3 h-3 bg-blue-400 border-none" />
      
      <div className="flex items-center space-x-3">
        <div className={`p-2 rounded-lg ${isTable ? 'bg-indigo-500/30 text-indigo-300' : isView ? 'bg-emerald-500/30 text-emerald-300' : 'bg-rose-500/30 text-rose-300'}`}>
          {isTable ? <Database size={18} /> : isView ? <Layers size={18} /> : <FileCode size={18} />}
        </div>
        <div>
          <p className="text-sm font-semibold text-white truncate max-w-[130px]">{label}</p>
          <p className="text-xs text-white/50 uppercase tracking-widest mt-0.5">{typeStr}</p>
        </div>
      </div>

      <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-blue-400 border-none" />
    </div>
  );
}
