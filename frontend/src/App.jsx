import { useState } from 'react'
import {HashRouter, Route, Routes} from "react-router-dom";
import routers from './router'
import Layout from './Layout';
import React from 'react';

function App() {

  return (
    <HashRouter>
			<Routes>
				{
					routers.map(({ trunk, ...config }) => {
						const _trunk = trunk();
						const isLazy = _trunk instanceof Promise; // 判断是否是懒加载组件
						const Trunk = isLazy ? React.lazy(() => _trunk) : _trunk;
						return (
							<Route
								key={config.key || config.path}
								path={config.path}
								exact={config.exact}
								element={
									(config.layout) ? (
										isLazy ? (
											<React.Suspense>
												<Layout>
													<Trunk />
												</Layout>
											</React.Suspense>
										) : (
											<Layout>
												<Trunk />
											</Layout>
										)
									) : (
										isLazy ? (
											<React.Suspense>
												<Trunk />
											</React.Suspense>
										) : (
											<Trunk />
										)
									)}
							/>
						)
					})
				}
			</Routes>
		</HashRouter>
  )
}

export default App
