import { useEffect } from 'react';
import {
  ReactFlow,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  BackgroundVariant,
  Position
} from '@xyflow/react';
import type { Node, Edge } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';
import CustomNode from './CustomNode';

const nodeTypes = {
  custom: CustomNode,
};

const dagreGraph = new dagre.graphlib.Graph();
dagreGraph.setDefaultEdgeLabel(() => ({}));

const getLayoutedElements = (nodes: Node[], edges: Edge[], direction = 'TB') => {
  const isHorizontal = direction === 'LR';
  dagreGraph.setGraph({ rankdir: direction });

  nodes.forEach((node: Node) => {
    dagreGraph.setNode(node.id, { width: 200, height: 80 });
  });

  edges.forEach((edge: Edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  dagre.layout(dagreGraph);

  const newNodes = nodes.map((node: Node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    const newNode = {
      ...node,
      targetPosition: isHorizontal ? Position.Left : Position.Top,
      sourcePosition: isHorizontal ? Position.Right : Position.Bottom,
      position: {
        x: nodeWithPosition.x - 200 / 2,
        y: nodeWithPosition.y - 80 / 2,
      },
      type: 'custom',
    };
    return newNode;
  });

  return { nodes: newNodes, edges };
};

function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  useEffect(() => {
    fetch('http://localhost:8080/api/dag')
      .then(res => res.json())
      .then(data => {
        const { nodes: layoutedNodes, edges: layoutedEdges } = getLayoutedElements(
          data.nodes,
          data.edges
        );
        setNodes(layoutedNodes);
        setEdges(layoutedEdges);
      })
      .catch(err => console.error("Failed to fetch DAG:", err));
  }, []);

  return (
    <div className="w-screen h-screen bg-slate-950 font-sans">
      {/* Header */}
      <div className="absolute top-0 left-0 w-full p-6 z-10 pointer-events-none">
        <h1 className="text-3xl font-bold text-white tracking-tight drop-shadow-lg">
          SQLForge <span className="text-blue-500">Explorer</span>
        </h1>
        <p className="text-slate-400 text-sm mt-1">Live Execution DAG Lineage</p>
      </div>

      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        className="bg-slate-950"
        defaultEdgeOptions={{
          style: { stroke: '#475569', strokeWidth: 2 },
          animated: true,
        }}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={2} color="#334155" />
        <Controls className="bg-slate-900 border-slate-800 fill-slate-300" />
      </ReactFlow>
    </div>
  );
}

export default App;
